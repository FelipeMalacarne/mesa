// Package sops encrypts YAML documents in the SOPS format for age
// recipients, so Mesa can add secrets to a GitOps repository that sops,
// KSOPS and Flux decrypt. It only encrypts: Mesa holds public keys, never
// the private one, and never reads back what it wrote.
//
// The format follows sops 3: every value is sealed with AES-256-GCM under a
// random data key, with its key path as additional data; a SHA-512 over the
// plaintext values, sealed the same way, guards the whole file; the data key
// is sealed with age for each recipient. Comments are dropped.
package sops

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	"go.yaml.in/yaml/v3"
)

// Version is the sops format version written in the metadata.
const Version = "3.11.0"

// unencryptedSuffix is sops' default: keys ending with it stay readable.
const unencryptedSuffix = "_unencrypted"

// Encrypt returns plain (one or more YAML documents, each a mapping) sealed
// for the age recipients ("age1…").
func Encrypt(plain []byte, recipients []string, now time.Time) ([]byte, error) {
	if len(recipients) == 0 {
		return nil, errors.New("sops: no age recipients")
	}
	var parsed []age.Recipient
	for _, r := range recipients {
		p, err := age.ParseX25519Recipient(strings.TrimSpace(r))
		if err != nil {
			return nil, fmt.Errorf("sops: recipient %q: %w", r, err)
		}
		parsed = append(parsed, p)
	}
	var docs []*yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(plain))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("sops: read YAML: %w", err)
		}
		if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
			return nil, errors.New("sops: every document must be a mapping")
		}
		docs = append(docs, doc.Content[0])
	}
	if len(docs) == 0 {
		return nil, errors.New("sops: nothing to encrypt")
	}

	dataKey := make([]byte, 32)
	if _, err := rand.Read(dataKey); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, 32)
	if err != nil {
		return nil, err
	}
	mac := sha512.New()
	for _, d := range docs {
		if err := seal(d, nil, gcm, mac); err != nil {
			return nil, err
		}
	}

	lastModified := now.UTC().Format(time.RFC3339)
	sealedMAC, err := sealValue(gcm, fmt.Sprintf("%X", mac.Sum(nil)), "str", lastModified)
	if err != nil {
		return nil, err
	}
	var keys []*yaml.Node
	for i, r := range parsed {
		var buf bytes.Buffer
		aw := armor.NewWriter(&buf)
		w, err := age.Encrypt(aw, r)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(dataKey); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		if err := aw.Close(); err != nil {
			return nil, err
		}
		enc := buf.String()
		if !strings.HasSuffix(enc, "\n") {
			enc += "\n"
		}
		keys = append(keys, mapping(
			"recipient", scalar(strings.TrimSpace(recipients[i])),
			"enc", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: enc, Style: yaml.LiteralStyle},
		))
	}
	meta := mapping(
		"age", &yaml.Node{Kind: yaml.SequenceNode, Content: keys},
		"lastmodified", scalar(lastModified),
		"mac", scalar(sealedMAC),
		"unencrypted_suffix", scalar(unencryptedSuffix),
		"version", scalar(Version),
	)

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(4)
	for _, d := range docs {
		d.Content = append(d.Content, scalar("sops"), meta)
		if err := enc.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{d}}); err != nil {
			return nil, err
		}
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// seal encrypts every scalar under n in place, in document order, adding
// its plaintext to the MAC. path is the chain of mapping keys above n.
func seal(n *yaml.Node, path []string, gcm cipher.AEAD, mac io.Writer) error {
	n.HeadComment, n.LineComment, n.FootComment = "", "", ""
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			k.HeadComment, k.LineComment, k.FootComment = "", "", ""
			if k.Kind != yaml.ScalarNode {
				return errors.New("sops: only scalar keys are supported")
			}
			if k.Value == "sops" && len(path) == 0 {
				return errors.New("sops: the document already has a sops key")
			}
			if err := seal(v, append(path[:len(path):len(path)], k.Value), gcm, mac); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if err := seal(item, path, gcm, mac); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		plain, typ, err := typed(n)
		if err != nil {
			return fmt.Errorf("sops: %s: %w", strings.Join(path, "."), err)
		}
		if typ == "" {
			return nil // null stays null, as sops leaves it
		}
		_, _ = mac.Write([]byte(plain))
		for _, k := range path {
			if strings.HasSuffix(k, unencryptedSuffix) {
				return nil
			}
		}
		if plain == "" {
			return nil // sops leaves empty strings as they are
		}
		v, err := sealValue(gcm, plain, typ, strings.Join(path, ":")+":")
		if err != nil {
			return err
		}
		*n = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
	default:
		return errors.New("sops: anchors and aliases are not supported")
	}
	return nil
}

// typed returns the bytes sops encrypts for a scalar and their sops type:
// strings as they are, booleans as True or False, numbers as written.
func typed(n *yaml.Node) (string, string, error) {
	switch n.ShortTag() {
	case "!!str":
		return n.Value, "str", nil
	case "!!null":
		return "", "", nil
	case "!!bool":
		b, err := strconv.ParseBool(strings.ToLower(n.Value))
		if err != nil {
			return "", "", err
		}
		if b {
			return "True", "bool", nil
		}
		return "False", "bool", nil
	case "!!int":
		i, err := strconv.Atoi(n.Value)
		if err != nil {
			return "", "", err
		}
		return strconv.Itoa(i), "int", nil
	case "!!float":
		f, err := strconv.ParseFloat(n.Value, 64)
		if err != nil {
			return "", "", err
		}
		return strconv.FormatFloat(f, 'f', -1, 64), "float", nil
	}
	return "", "", fmt.Errorf("unsupported YAML type %s; quote the value", n.ShortTag())
}

func sealValue(gcm cipher.AEAD, plain, typ, additional string) (string, error) {
	iv := make([]byte, 32)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	out := gcm.Seal(nil, iv, []byte(plain), []byte(additional))
	tag := out[len(out)-aes.BlockSize:]
	b64 := base64.StdEncoding
	return fmt.Sprintf("ENC[AES256_GCM,data:%s,iv:%s,tag:%s,type:%s]",
		b64.EncodeToString(out[:len(out)-aes.BlockSize]), b64.EncodeToString(iv), b64.EncodeToString(tag), typ), nil
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func mapping(kv ...any) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(kv); i += 2 {
		m.Content = append(m.Content, scalar(kv[i].(string)), kv[i+1].(*yaml.Node))
	}
	return m
}
