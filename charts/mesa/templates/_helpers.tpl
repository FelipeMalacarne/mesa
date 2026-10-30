{{/*
Expand the name of the chart.
*/}}
{{- define "mesa.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "mesa.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "mesa.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "mesa.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "mesa.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mesa.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
The declared connections as TOML (MESA_CONNECTIONS). Values are written as
JSON scalars, which TOML reads the same; numbers become integers; a nested
map (gitops) becomes its own table.
*/}}
{{- define "mesa.tomlValue" -}}
{{- if or (kindIs "float64" .) (kindIs "int64" .) (kindIs "int" .) }}{{ int64 . }}{{ else }}{{ toJson . }}{{ end -}}
{{- end }}

{{- define "mesa.connectionsToml" -}}
# Declared by the Helm release {{ .Release.Name }}; change them in its values.
{{- range $name, $conn := .Values.connections }}

[connections.{{ $name | toJson }}]
{{- range $key, $value := $conn }}
{{- if not (kindIs "map" $value) }}
{{ $key }} = {{ include "mesa.tomlValue" $value }}
{{- end }}
{{- end }}
{{- range $key, $value := $conn }}
{{- if kindIs "map" $value }}

[connections.{{ $name | toJson }}.{{ $key }}]
{{- range $k, $v := $value }}
{{ $k }} = {{ include "mesa.tomlValue" $v }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
