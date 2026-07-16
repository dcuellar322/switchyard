import { chmodSync, existsSync, mkdirSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const repository = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const rustcCandidate = join(homedir(), ".cargo", "bin", process.platform === "win32" ? "rustc.exe" : "rustc");
const rustc = process.env.RUSTC || (existsSync(rustcCandidate) ? rustcCandidate : "rustc");
const target = run(rustc, ["--print", "host-tuple"]).trim();
const metadata = JSON.parse(readFileSync(join(repository, "package.json"), "utf8"));
const version = process.env.SWITCHYARD_VERSION || metadata.version;
const commit = run("git", ["-C", repository, "rev-parse", "--short=12", "HEAD"], true).trim() || "unknown";
const builtAt = new Date().toISOString().replace(/\.\d{3}Z$/, "Z");
const extension = process.platform === "win32" ? ".exe" : "";
const output = join(repository, "desktop", "src-tauri", "binaries", `switchyard-${target}${extension}`);
const cache = join(repository, ".cache", "go-build");

mkdirSync(dirname(output), { recursive: true });
mkdirSync(cache, { recursive: true });
const linkerFlags = [
  `-X switchyard.dev/switchyard/internal/foundation/buildinfo.version=${version}`,
  `-X switchyard.dev/switchyard/internal/foundation/buildinfo.commit=${commit}`,
  `-X switchyard.dev/switchyard/internal/foundation/buildinfo.builtAt=${builtAt}`,
].join(" ");

run("go", ["build", "-trimpath", "-ldflags", linkerFlags, "-o", output, "./cmd/switchyard"], false, {
  ...process.env,
  GOCACHE: cache,
}, repository);
if (process.platform !== "win32") chmodSync(output, 0o755);

function run(executable, arguments_, allowFailure = false, environment = process.env, cwd = repository) {
  const result = spawnSync(executable, arguments_, { cwd, env: environment, encoding: "utf8", stdio: ["ignore", "pipe", "inherit"] });
  if (result.status !== 0 && !allowFailure) {
    throw new Error(`${executable} failed with exit code ${result.status ?? "unknown"}`);
  }
  return result.stdout || "";
}
