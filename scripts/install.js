const fs = require("node:fs");
const https = require("node:https");
const crypto = require("node:crypto");
const os = require("node:os");
const path = require("node:path");

const packageJson = require("../package.json");

const version = `v${packageJson.version}`;
const baseUrl = process.env.AIGIT_DOWNLOAD_BASE_URL || "https://github.com/Satari4545/ai-git-releases/releases/download";
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
const url = `${baseUrl}/${version}/${assetName}`;
const vendorDir = path.join(__dirname, "..", "vendor");
const outputPath = path.join(vendorDir, platform === "windows" ? "agit.exe" : "agit");

fs.mkdirSync(vendorDir, { recursive: true });

download(url, outputPath).then(() => {
  return verifyChecksum(`${url}.sha256`, outputPath);
}).then(() => {
  if (platform !== "windows") {
    fs.chmodSync(outputPath, 0o755);
  }
  console.log(`Installed ${assetName} (checksum verified)`);
}).catch((error) => {
  console.error(`Failed to install agit binary from ${url}`);
  console.error(error.message);
  process.exit(1);
});

function verifyChecksum(checksumUrl, outputPath) {
  return downloadToString(checksumUrl).then((body) => {
    const expected = body.trim().split(/\s+/)[0];
    return sha256File(outputPath).then((actual) => {
      if (!expected || expected !== actual) {
        fs.unlinkSync(outputPath);
        throw new Error(`Checksum mismatch for ${assetName}: expected ${expected || "(unreadable)"}, got ${actual}`);
      }
    });
  });
}

function sha256File(filePath) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256");
    const stream = fs.createReadStream(filePath);
    stream.on("error", reject);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("end", () => resolve(hash.digest("hex")));
  });
}

function downloadToString(url) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, (response) => {
      if ([301, 302, 303, 307, 308].includes(response.statusCode)) {
        response.resume();
        downloadToString(response.headers.location).then(resolve, reject);
        return;
      }
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`HTTP ${response.statusCode}`));
        return;
      }
      let body = "";
      response.setEncoding("utf8");
      response.on("data", (chunk) => { body += chunk; });
      response.on("end", () => resolve(body));
      response.on("error", reject);
    });
    request.on("error", reject);
  });
}

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
