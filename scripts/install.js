const fs = require("node:fs");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");

const packageJson = require("../package.json");

const repo = "Satari4545/ai-git";
const version = `v${packageJson.version}`;
const platformMap = {
  linux: "linux",
  darwin: "darwin",
  win32: "windows"
};
const archMap = {
  x64: "amd64",
  arm64: "arm64"
};

const platform = platformMap[os.platform()];
const arch = archMap[os.arch()];

if (!platform || !arch) {
  throw new Error(`Unsupported platform: ${os.platform()} ${os.arch()}`);
}

const assetName = `agit-${platform}-${arch}${platform === "windows" ? ".exe" : ""}`;
const url = `https://github.com/${repo}/releases/download/${version}/${assetName}`;
const vendorDir = path.join(__dirname, "..", "vendor");
const outputPath = path.join(vendorDir, platform === "windows" ? "agit.exe" : "agit");

fs.mkdirSync(vendorDir, { recursive: true });

download(url, outputPath).then(() => {
  if (platform !== "windows") {
    fs.chmodSync(outputPath, 0o755);
  }
  console.log(`Installed ${assetName}`);
}).catch((error) => {
  console.error(`Failed to install agit binary from ${url}`);
  console.error(error.message);
  process.exit(1);
});

function download(url, outputPath) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, (response) => {
      if ([301, 302, 303, 307, 308].includes(response.statusCode)) {
        response.resume();
        download(response.headers.location, outputPath).then(resolve, reject);
        return;
      }

      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`HTTP ${response.statusCode}`));
        return;
      }

      const file = fs.createWriteStream(outputPath);
      response.pipe(file);
      file.on("finish", () => file.close(resolve));
      file.on("error", reject);
    });

    request.on("error", reject);
  });
}
