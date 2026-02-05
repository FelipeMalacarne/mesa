import { OpenAPI } from "@/api";

export const configureApiClient = () => {
  const base = import.meta.env.VITE_API_BASE;
  if (!base) {
    console.warn("VITE_API_BASE is not set; API client will use relative URLs");
  }
  OpenAPI.BASE = base ?? "";
  OpenAPI.WITH_CREDENTIALS = false;
  // OpenAPI.TOKEN = async () => {
  //   const user = firebaseAuth.currentUser;
  //   if (user) {
  //     try {
  //       return await user.getIdToken();
  //     } catch {
  //       // fall through
  //     }
  //   }
  //   return localStorage.getItem("authToken") ?? "";
  // };
};
