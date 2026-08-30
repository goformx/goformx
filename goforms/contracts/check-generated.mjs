import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const cwd = fileURLToPath(new URL("../", import.meta.url));
execFileSync("git", ["diff", "--exit-code", "--", "contracts/generated"], { cwd, stdio: "inherit" });
const untracked = execFileSync("git", ["ls-files", "--others", "--exclude-standard", "--", "contracts/generated"], { cwd }).toString().trim();
if (untracked) throw new Error(`Generated artifacts must be committed:\n${untracked}`);
