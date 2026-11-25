#!/usr/bin/env python3
"""Draft labels for an agent eval case from its raw Playwright snapshot.

Reads core/internal/agent/eval/testdata/<case>/page.yaml.gz and writes a draft
expected.json next to it with every listing found on the page (title, price,
currency, URL path) and a first guess at whether it is the product itself.
The guess is a starting point: review expected.json by hand before trusting it.

It parses the raw snapshot, not the agent's compacted view, so the labels do not
depend on the pipeline being evaluated.

    python3 tools/eval/label_draft.py <case-dir> "<product>" <store>
"""
import gzip
import json
import re
import sys
from pathlib import Path

PRICE = re.compile(r"(SGD|S\$|USD|US\$|\$|JPY|￥|¥|CNY)\s?([\d,]+(?:\.\d{1,2})?)")
NOT_A_PRICE = ("市场价", "原价", "配送费", "运费", "shipping", "List", "Typical", "积分", "points")
CURRENCY = {"SGD": "SGD", "S$": "SGD", "USD": "USD", "US$": "USD", "$": "USD", "JPY": "JPY", "CNY": "CNY"}

# Words that mean a listing is something other than the product itself.
NOT_PRODUCT = re.compile(
    # whole computers that contain the product
    r"laptop|notebook|gaming pc|desktop|游戏电脑|迷你电脑|台式电脑|外置显卡|external gpu|笔记本|游戏本|ノート|パソコン|主机|整机|台式|服务器|工作站|\bnuc\b|"
    # accessories and parts sold for the product
    r"bracket|holder|support|支架|支撑|cable|电缆|数据线|电源线|延长线|音频线|ケーブル|cord|riser|extender|延长|"
    r"adapter|适配器|power supply|电源|psu|12vhpwr|ピン|"
    r"\bcase\b|机箱|thermal|硅脂|gasket|shield|water ?cooler|water ?block|水冷头|cover|保护套|硬壳|耳机袋|"
    r"ear ?pads?|earmuffs?|cushions?|耳罩|耳垫|イヤーパッド|イヤークッション|repair|修理|stand|"
    r"headband|头带|防汗|sweat|silicone|硅胶|"
    r"bundle|套装",
    re.I,
)


def unquote(s):
    m = re.search(r'"((?:[^"\\]|\\.)*)"', s)
    return m.group(1).replace('\\"', '"') if m else None


def currency_for(symbol, store):
    if symbol in ("￥", "¥"):
        return "JPY" if store == "amazon-jp" else "CNY"
    return CURRENCY[symbol]


def find_price(lines, start, stop, store):
    """First real price in lines[start:stop], joining fragments like '¥' '27898.' '04'."""
    i = start
    while i < stop:
        line = lines[i]
        if any(w in line for w in NOT_A_PRICE):
            i += 1
            continue
        m = PRICE.search(line)
        if m:
            return m.group(2).replace(",", ""), currency_for(m.group(1), store)
        # SMZDM: a lone currency symbol followed by number fragments
        if re.search(r":\s*[￥¥]\s*$", line):
            parts = []
            for nxt in lines[i + 1 : i + 4]:
                v = re.search(r':\s*"?([\d.,]+)"?\s*$', nxt)
                if not v:
                    break
                parts.append(v.group(1))
            if parts:
                return "".join(parts).replace(",", "").rstrip("."), currency_for("¥", store)
        i += 1
    return None, None


def amazon_listings(lines, store):
    out = []
    heads = [i for i, l in enumerate(lines) if re.match(r"\s*- '?heading \"", l) and "[level=2]" in l]
    brand_titles = set()
    for n, i in enumerate(heads):
        if i in brand_titles:
            continue  # already used as the title of the brand heading above it
        title = unquote(lines[i])
        title_at = i
        if title and len(title) < 15:
            # A brand heading ("Sony"); the product name is the next named line.
            for k in range(i + 1, min(len(lines), i + 8)):
                t = unquote(lines[k]) if re.match(r"\s*- '?(heading|link|generic) \"", lines[k]) else None
                if t and len(t) >= 15:
                    title, title_at = t, k
                    brand_titles.add(k)
                    break
        if not title or len(title) < 15:
            continue
        later = [h for h in heads if h > title_at and h not in brand_titles]
        nxt = later[0] if later else min(len(lines), title_at + 60)
        # The card's own link is the first product link after the card starts; colour
        # options listed between it and the title link to other variants.
        indent = lambda line: len(line) - len(line.lstrip())
        start = next((j for j in range(i, max(-1, i - 60), -1)
                      if re.match(r"\s*- listitem\b", lines[j]) and indent(lines[j]) < indent(lines[title_at])),
                     max(0, i - 40))
        url = None
        for j in range(start, title_at + 1):
            m = re.search(r"/url: (\S*/dp/([A-Z0-9]{10}))", lines[j])
            if m:
                url = "/dp/" + m.group(2)
                break
        price, cur = find_price(lines, title_at + 1, nxt, store)
        if url:
            # A card without a price ("see options") is kept with price null: an offer
            # for it can only carry a made-up price.
            out.append({"title": title, "price": price, "currency": cur, "url_path": url})
    return out


def smzdm_listings(lines, store):
    out = []
    for i, l in enumerate(lines):
        if not re.match(r"\s*- link\b", l):
            continue
        if i + 1 >= len(lines):
            continue
        m = re.search(r"/url: https?://www\.smzdm\.com(/p/\d+/?)$", lines[i + 1].strip())
        if not m:
            continue
        title = unquote(l)
        if not title:
            # The title is a text child of the link: "- link:" "- /url: ..." "- text: TITLE"
            for nxt in lines[i + 2 : i + 5]:
                t = re.match(r"\s*- text: (.+)$", nxt)
                if t:
                    title = t.group(1).strip().strip('"')
                    break
        if not title or len(title) < 8:
            continue
        price, cur = find_price(lines, i + 2, min(len(lines), i + 14), store)
        if price:
            out.append({"title": title, "price": price, "currency": cur, "url_path": m.group(1).rstrip("/")})
    return out


def main():
    case_dir, product, store = Path(sys.argv[1]), sys.argv[2], sys.argv[3]
    lines = gzip.decompress((case_dir / "page.yaml.gz").read_bytes()).decode().split("\n")
    listings = smzdm_listings(lines, store) if store == "smzdm" else amazon_listings(lines, store)

    # The model number (the word with digits) must appear; brand names are often
    # written in another script (索尼 for Sony), so they are not required.
    terms = [t for t in product.lower().split() if any(c.isdigit() for c in t)]
    seen, labelled = set(), []
    for x in listings:
        if x["url_path"] in seen:
            continue
        seen.add(x["url_path"])
        compact_title = re.sub(r"\s+", "", x["title"].lower())
        named = all(re.sub(r"\s+", "", t) in compact_title for t in terms)
        x["is_product"] = bool(named and not NOT_PRODUCT.search(x["title"]))
        # A title naming several models (e.g. "RTX 5090D V2 5080 5070TI") could be
        # any of them. Ambiguous listings count neither for nor against the agent.
        # Graphics card model numbers look like 4090, 5080, 5070: digit, 0, digit, 0.
        models = set(re.findall(r"(?<!\d)[1-9]0[3-9]0(?!\d)", x["title"]))
        x["ambiguous"] = bool(x["is_product"] and len(models) > 1)
        labelled.append(x)

    # Hand decisions that override the rules, with a reason for each.
    overrides_path = case_dir / "overrides.json"
    overrides = json.loads(overrides_path.read_text()) if overrides_path.exists() else {}
    for x in labelled:
        o = overrides.get(x["url_path"])
        if o:
            x["is_product"] = o["is_product"]
            x["ambiguous"] = o.get("ambiguous", False)
            x["reviewed"] = o["reason"]

    expected = {
        "product": product,
        "store": store,
        "note": "Generated by tools/eval/label_draft.py from page.yaml.gz; hand decisions are in overrides.json.",
        "listings": labelled,
    }
    (case_dir / "expected.json").write_text(json.dumps(expected, ensure_ascii=False, indent=2) + "\n")
    yes = sum(x["is_product"] for x in labelled)
    print(f"{case_dir.name}: {len(labelled)} listings, {yes} marked as the product")
    for x in labelled:
        mark = "?" if x["ambiguous"] else ("Y" if x["is_product"] else ".")
        print(f"  {mark} {x['currency'] or '-'} {x['price'] or 'no price':>10}  {x['url_path']:<14} {x['title'][:70]}")


if __name__ == "__main__":
    main()
