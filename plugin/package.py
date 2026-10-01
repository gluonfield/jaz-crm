import argparse
import json
from pathlib import Path
from urllib.parse import urlsplit
from zipfile import ZIP_DEFLATED, ZipFile

parser = argparse.ArgumentParser(description="Package this plugin for an existing HTTPS MCP endpoint.")
parser.add_argument("endpoint", help="Deployed MCP URL, including /mcp")
parser.add_argument("archive", type=Path, help="Output ZIP path outside the plugin directory")
args = parser.parse_args()
endpoint = args.endpoint.strip()
url = urlsplit(endpoint)
if url.scheme != "https" or not url.hostname or url.username or url.password or url.query or url.fragment:
    parser.error("endpoint must be an HTTPS URL without credentials, query or fragment")
root = Path(__file__).resolve().parent
archive = args.archive.resolve()
if archive.is_relative_to(root):
    parser.error("write the archive outside the plugin directory")
manifest = json.loads((root / "plugin.json").read_text())
name = manifest["name"]
mcp = {
    "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
    "mcpServers": {name: {"type": "streamable-http", "url": endpoint}},
}
with ZipFile(archive, "w", compression=ZIP_DEFLATED) as bundle:
    bundle.write(root / "plugin.json", f"{name}/plugin.json")
    for asset in sorted((root / "assets").iterdir()):
        bundle.write(asset, f"{name}/assets/{asset.name}")
    bundle.writestr(f"{name}/mcp.json", json.dumps(mcp, indent=2) + "\n")
print(archive)
