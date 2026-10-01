#!/usr/bin/env python3
"""
Reusable test script to verify SimpleFS ZIP upload, extraction, security validations, and folder ZIP downloads.
"""

import sys
import io
import os
import zipfile
import urllib.request
import urllib.error

BASE_URL = os.getenv("SIMPLEFS_URL", "http://127.0.0.1:8080")

def test_download_folder_as_zip(folder_path="sample_docs"):
    print(f"[*] Testing download of folder '{folder_path}' as ZIP...")
    url = f"{BASE_URL}/api/download-folder?path={folder_path}"
    req = urllib.request.Request(url)
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200, f"Expected 200, got {resp.status}"
        assert resp.headers.get("Content-Type") == "application/zip", f"Unexpected content-type: {resp.headers.get('Content-Type')}"
        data = resp.read()
        zf = zipfile.ZipFile(io.BytesIO(data))
        namelist = zf.namelist()
        print(f"    [+] Successfully downloaded {len(data)} bytes ZIP.")
        print(f"    [+] Entries in archive: {namelist}")
        assert len(namelist) > 0, "ZIP archive should not be empty"

def test_upload_valid_zip():
    print("[*] Testing upload and extraction of valid ZIP...")
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("notes.txt", "Automated test notes")
        zf.writestr("sub/config.ini", "[app]\nversion=1.0")
    zip_bytes = buf.getvalue()

    boundary = "----WebKitFormBoundarySimpleFSValidTest"
    body = (
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="path"\r\n\r\n\r\n'
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="zip_file"; filename="reusable_test.zip"\r\n'
        "Content-Type: application/zip\r\n\r\n"
    ).encode("utf-8") + zip_bytes + f"\r\n--{boundary}--\r\n".encode("utf-8")

    req = urllib.request.Request(
        f"{BASE_URL}/api/upload-zip",
        data=body,
        headers={
            "Content-Type": f"multipart/form-data; boundary={boundary}",
            "Origin": BASE_URL,
        }
    )

    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200, f"Expected 200 OK, got {resp.status}"
        print("    [+] Uploaded and extracted 'reusable_test.zip' successfully.")

def test_security_zip_slip_rejection():
    print("[*] Testing security validation: Zip Slip (path traversal) rejection...")
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr("../../dangerous_escape.txt", "should never be written")
    zip_bytes = buf.getvalue()

    boundary = "----WebKitFormBoundarySimpleFSSecurityTest"
    body = (
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="path"\r\n\r\n\r\n'
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="zip_file"; filename="malicious_slip.zip"\r\n'
        "Content-Type: application/zip\r\n\r\n"
    ).encode("utf-8") + zip_bytes + f"\r\n--{boundary}--\r\n".encode("utf-8")

    req = urllib.request.Request(
        f"{BASE_URL}/api/upload-zip",
        data=body,
        headers={
            "Content-Type": f"multipart/form-data; boundary={boundary}",
            "Origin": BASE_URL,
        }
    )

    try:
        urllib.request.urlopen(req)
        assert False, "Security violation: Malicious zip with path traversal was not rejected!"
    except urllib.error.HTTPError as e:
        assert e.code == 400, f"Expected 400 Bad Request, got {e.code}"
        error_msg = e.read().decode("utf-8", errors="ignore")
        print(f"    [+] Correctly rejected by server with HTTP 400: {error_msg.strip()}")

def main():
    print(f"=== Running SimpleFS ZIP Feature Tests against {BASE_URL} ===")
    test_upload_valid_zip()
    test_download_folder_as_zip("reusable_test")
    test_security_zip_slip_rejection()
    print("=== All ZIP features and security checks passed successfully! ===")

if __name__ == "__main__":
    main()
