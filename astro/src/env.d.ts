/// <reference types="astro/client" />

interface ImportMetaEnv {
  readonly PUBLIC_CI_API_HOST: string;
  readonly PUBLIC_CI_API_PORT: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
