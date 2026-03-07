import { defineConfig } from "orval";

export default defineConfig({
  mesaHooks: {
    input: {
      target: "../openapi.yaml",
    },
    output: {
      mode: "tags-split",
      target: "src/api",
      client: "react-query",
      override: {
        fetch: {
          includeHttpResponseReturnType: false,
        },
        mutator: {
          path: "src/lib/fetch-client.ts",
          name: "customInstance",
        },
        query: {
          useQuery: true,
          useMutation: true,
        },
      },
    },
  },
  mesaZod: {
    input: {
      target: "../openapi.yaml",
    },
    output: {
      client: "zod",
      target: "src/api/zod.gen.ts",
      fileExtension: ".ts",
    },
  },
});
