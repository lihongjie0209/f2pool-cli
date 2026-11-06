#!/usr/bin/env python3
"""Extract V2 endpoint names and request fields from the saved official HTML."""
import html
import json
import re
from pathlib import Path

root = Path(__file__).resolve().parents[1]
source = (root / "docs/f2pool-api-v2-en.html").read_text()


def plain(value):
    return html.unescape(re.sub(r"<[^>]+>", "", value)).strip()


catalog = []
section = ""
for item in re.finditer(r"<(h1|h2)\b[^>]*>(.*?)</\1>(.*?)(?=<h[12]\b|</body)", source, re.S):
    tag, title, body = item.groups()
    if tag == "h1":
        section = plain(title)
        continue
    match = re.search(r"<pre[^>]*>(.*?)</pre>", body, re.S)
    if not match:
        continue
    route = re.fullmatch(r"POST\s+(/v2/[a-z_/-]+)", plain(match.group(1)))
    if not route:
        continue
    request = re.search(r"<h4[^>]*>.*?Request Parameter</h4>\s*(<table>.*?</table>)", body, re.S)
    fields = []
    if request:
        rows = re.findall(r"<tr>(.*?)</tr>", request.group(1), re.S)[1:]
        for row in rows:
            cells = [plain(cell) for cell in re.findall(r"<td[^>]*>(.*?)</td>", row, re.S)]
            if cells:
                fields.append({"name": cells[0], "type": cells[1], "repeated": cells[2] == "repeated", "description": cells[3]})
    catalog.append({"section": section, "title": plain(title), "path": route.group(1), "fields": fields})

output = root / "internal/catalog/endpoints.json"
output.parent.mkdir(parents=True, exist_ok=True)
output.write_text(json.dumps(catalog, indent=2, ensure_ascii=False) + "\n")
print(f"{len(catalog)} endpoints -> {output}")
