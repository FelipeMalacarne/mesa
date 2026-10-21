import { defineConfig } from "orval";

// The generated client lives in src/lib/api (one folder per OpenAPI tag).
// Regenerate with `pnpm run codegen` whenever api/openapi.yaml changes; never edit the output.
export default defineConfig({
  mesa: {
    input: {
      // Written by scripts/prepare-spec.mjs from ../api/openapi.yaml (see that file for why).
      target: ".orval/openapi.yaml",
    },
    output: {
      mode: "tags-split",
      target: "src/lib/api",
      client: "react-query",
      httpClient: "fetch",
      override: {
        mutator: {
          path: "src/lib/api/fetch.ts",
          name: "customFetch",
        },
        fetch: {
          includeHttpResponseReturnType: false,
        },
        query: {
          useQuery: true,
          useMutation: true,
        },
      },
    },
  },
});
