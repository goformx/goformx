import { cpSync } from "node:fs";

// Preserve relative companion-schema links in the downloadable OpenAPI bundle.
for (const directory of ["schema", "auth"]) {
  cpSync(new URL(`./${directory}`, import.meta.url), new URL(`./generated/${directory}`, import.meta.url), { recursive: true });
}
