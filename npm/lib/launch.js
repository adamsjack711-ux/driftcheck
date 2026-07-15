'use strict';

const { spawnSync } = require('child_process');
const path = require('path');
const fs = require('fs');

function isExecutable(p) {
  try {
    fs.accessSync(p, fs.constants.X_OK);
    return fs.statSync(p).isFile();
  } catch {
    return false;
  }
}

function launch(name) {
  const override = process.env[`${name.toUpperCase()}_BIN`];
  const bundled = path.join(
    __dirname,
    '..',
    'dist',
    `${process.platform}-${process.arch}`,
    name
  );

  const bin = [override, bundled].filter(Boolean).find(isExecutable);

  if (!bin) {
    console.error(
      `${name}: no prebuilt binary for ${process.platform}-${process.arch}.`
    );
    console.error(
      'driftcheck ships binaries for macOS and Linux (arm64 + x64). On other'
    );
    console.error(
      `platforms, build from source with go and point ${name.toUpperCase()}_BIN at the binary.`
    );
    process.exit(2);
  }

  const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
  if (result.error) {
    console.error(`${name}: ${result.error.message}`);
    process.exit(2);
  }
  // Exit codes are the contract (0 clean, 1 drift, 2 error) — pass through
  // untouched, and map a signal death to 2 (error), never 1 (drift).
  process.exit(result.status ?? 2);
}

module.exports = launch;
