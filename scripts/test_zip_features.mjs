/**
 * Reusable test script (Node.js) to verify SimpleFS ZIP upload, extraction,
 * security validations, and folder ZIP downloads inside devcontainer or host.
 */

const BASE_URL = process.env.SIMPLEFS_URL || 'http://127.0.0.1:8080';

async function testDownloadFolder() {
  console.log("[*] Testing download of folder as ZIP...");
  const res = await fetch(`${BASE_URL}/api/download-folder?path=reusable_test`);
  if (!res.ok) {
    throw new Error(`Failed to download folder: ${res.status}`);
  }
  const contentType = res.headers.get("content-type");
  const contentDisp = res.headers.get("content-disposition");
  if (!contentType?.includes("application/zip")) {
    throw new Error(`Invalid content-type: ${contentType}`);
  }
  const buf = await res.arrayBuffer();
  console.log(`    [+] Received ${buf.byteLength} bytes ZIP (Content-Disposition: ${contentDisp})`);
}

async function testZipSlipRejection() {
  console.log("[*] Testing security validation: Zip Slip rejection...");
  // Minimal ZIP header with path traversal entry
  const dummySlipZip = Buffer.from(
    "UEsDBBQAAAAIAAAAAAAAAAAAAAAAAAAAAAAUAAAALi4vLi4vZXZpbF9zbGlwLnR4dGV2aWxfZGF0YVBLAQIUABQAAAAIAAAAAAAAAAAAAAAAAAAAAAAUAAAAAAAAAAAAIAAAAAAAAAC4uLy4uL2V2aWxfc2xpcC50eHRQSwUGAAAAAAEAAQBAAAAARgAAAAAA",
    "base64"
  );

  const formData = new FormData();
  formData.append("path", "");
  formData.append("zip_file", new Blob([dummySlipZip], { type: "application/zip" }), "evil_slip.zip");

  const res = await fetch(`${BASE_URL}/api/upload-zip`, {
    method: "POST",
    headers: {
      Origin: BASE_URL,
    },
    body: formData,
  });

  if (res.status === 400) {
    const text = await res.text();
    console.log(`    [+] Correctly rejected by server with HTTP 400: ${text.trim()}`);
  } else {
    throw new Error(`Expected 400 Bad Request, got status ${res.status}`);
  }
}

async function main() {
  console.log(`=== Running SimpleFS ZIP Node.js Tests against ${BASE_URL} ===`);
  await testDownloadFolder();
  await testZipSlipRejection();
  console.log("=== Node.js ZIP validation tests passed successfully! ===");
}

main().catch((err) => {
  console.error("Test failed:", err);
  process.exit(1);
});
