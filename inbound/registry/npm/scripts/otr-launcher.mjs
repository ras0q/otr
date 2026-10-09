#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { readdirSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const pkg = require(join(here, "..", "package.json"));

function platformSuffix() {
	const os =
		process.platform === "win32"
			? "windows"
			: process.platform === "darwin"
				? "darwin"
				: "linux";
	const arch = process.arch === "x64" ? "amd64" : process.arch;
	return `${os}-${arch}`;
}

const suffix = `--${platformSuffix()}`;
const dep = Object.keys(pkg.optionalDependencies ?? {}).find((name) =>
	name.endsWith(suffix),
);
if (!dep) {
	console.error(`otr: no platform package for ${platformSuffix()}`);
	process.exit(1);
}

const root = dirname(require.resolve(`${dep}/package.json`));
const assetDir = join(root, "asset");
const assets = readdirSync(assetDir);
if (assets.length === 0) {
	console.error(`otr: no asset in ${dep}`);
	process.exit(1);
}

const bin = join(assetDir, assets[0]);
const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status ?? 1);
