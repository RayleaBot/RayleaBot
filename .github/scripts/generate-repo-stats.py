#!/usr/bin/env python3
"""Generate repository-specific commit stats SVGs for README embedding.

Reads commits from the GitHub API for the current repository and writes:
- dist/repo-activity-line.svg   animated monthly commit line chart
- dist/repo-activity-heatmap.svg weekly commit heatmap (GitHub-style)

Environment variables:
    GITHUB_TOKEN    optional GitHub token to raise API rate limits
    REPO_OWNER      repository owner/organization
    REPO_NAME       repository name
"""

from __future__ import annotations

import json
import os
import re
import sys
import urllib.error
import urllib.request
from collections import defaultdict
from datetime import date, datetime, timedelta, timezone
from pathlib import Path
from typing import Dict, List, Tuple

OUT_DIR = Path("dist")
OUT_DIR.mkdir(parents=True, exist_ok=True)

REPO_OWNER = os.environ.get("REPO_OWNER", "RayleaBot")
REPO_NAME = os.environ.get("REPO_NAME", "RayleaBot")
REPO = f"{REPO_OWNER}/{REPO_NAME}"

TOKEN = os.environ.get("GITHUB_TOKEN", "")
API_BASE = f"https://api.github.com/repos/{REPO}"

# Theme: dark-first, supports prefers-color-scheme via CSS variables where useful.
COLORS = {
    "bg_dark": "#0d1117",
    "bg_light": "#ffffff",
    "grid": "#21262d",
    "grid_light": "#e6edf0",
    "text": "#8b949e",
    "text_light": "#57606a",
    "line": "#58a6ff",
    "area_top": "#58a6ff",
    "area_bottom": "#58a6ff33",
    "heat": ["#161b22", "#0e4429", "#006d32", "#26a641", "#39d353"],
    "heat_light": ["#ebedf0", "#9be9a8", "#40c463", "#30a14e", "#216e39"],
}


def api_get(url: str) -> Tuple[dict, dict]:
    req = urllib.request.Request(url)
    if TOKEN:
        req.add_header("Authorization", f"Bearer {TOKEN}")
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("X-GitHub-Api-Version", "2022-11-28")
    req.add_header("User-Agent", "rayleabot-repo-stats")
    with urllib.request.urlopen(req, timeout=30) as resp:
        body = json.loads(resp.read().decode("utf-8"))
        headers = dict(resp.headers)
        return body, headers


def fetch_commits_since(since: datetime) -> List[datetime]:
    """Fetch commit timestamps since `since` using the commits API."""
    since_iso = since.replace(microsecond=0).isoformat()
    commits: List[datetime] = []
    page = 1
    while True:
        url = f"{API_BASE}/commits?since={since_iso}&per_page=100&page={page}"
        try:
            data, headers = api_get(url)
        except urllib.error.HTTPError as exc:
            print(f"GitHub API error: {exc.code} {exc.reason}", file=sys.stderr)
            try:
                print(exc.read().decode("utf-8", errors="ignore"), file=sys.stderr)
            except Exception:
                pass
            raise
        if not data:
            break
        for commit in data:
            raw = (
                commit.get("commit", {})
                .get("committer", {})
                .get("date")
                or commit.get("commit", {}).get("author", {}).get("date")
            )
            if raw:
                commits.append(datetime.fromisoformat(raw.replace("Z", "+00:00")))
        # GitHub returns the Link header for pagination; stop if absent.
        link = headers.get("Link", "")
        if 'rel="next"' not in link:
            break
        page += 1
        if page > 50:  # safety cap
            break
    return commits


def monthly_counts(commits: List[datetime]) -> Dict[str, int]:
    counts: Dict[str, int] = defaultdict(int)
    for dt in commits:
        key = dt.strftime("%Y-%m")
        counts[key] += 1
    return counts


def daily_counts(commits: List[datetime]) -> Dict[date, int]:
    counts: Dict[date, int] = defaultdict(int)
    for dt in commits:
        d = dt.astimezone(timezone.utc).date()
        counts[d] += 1
    return counts


def heatmap_buckets(daily: Dict[date, int]) -> Tuple[List[Tuple[date, int]], int, int]:
    """Return (date, count) list for the last 53 complete weeks plus current week."""
    today = date.today()
    # Start from the Monday 52 weeks ago.
    start = today - timedelta(days=today.weekday() + 52 * 7)
    days: List[Tuple[date, int]] = []
    current = start
    max_count = 0
    while current <= today:
        c = daily.get(current, 0)
        days.append((current, c))
        max_count = max(max_count, c)
        current += timedelta(days=1)
    return days, len(days) // 7 + (1 if len(days) % 7 else 0), max_count


def color_for_count(count: int, max_count: int, palette: List[str]) -> str:
    if count == 0:
        return palette[0]
    if max_count <= 1:
        return palette[-1]
    level = min(4, max(1, int(count / max_count * 4)))
    return palette[level]


def escape_xml(text: str) -> str:
    return (
        text.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace('"', "&quot;")
    )


def build_line_chart(monthly: Dict[str, int]) -> str:
    """Build an animated SVG line chart of monthly commits."""
    # Last 12 complete months + current month.
    today = date.today()
    months: List[str] = []
    for i in range(11, -1, -1):
        d = today.replace(day=1) - timedelta(days=i * 30)
        months.append(d.strftime("%Y-%m"))
    # Recompute from actual month boundaries.
    months = []
    cursor = today.replace(day=1)
    for _ in range(12):
        months.append(cursor.strftime("%Y-%m"))
        if cursor.month == 1:
            cursor = cursor.replace(year=cursor.year - 1, month=12)
        else:
            cursor = cursor.replace(month=cursor.month - 1)
    months.reverse()

    values = [monthly.get(m, 0) for m in months]
    max_val = max(values) or 1

    width = 820
    height = 220
    padding = {"top": 40, "right": 30, "bottom": 40, "left": 50}
    chart_w = width - padding["left"] - padding["right"]
    chart_h = height - padding["top"] - padding["bottom"]

    def x(i: int) -> float:
        return padding["left"] + (i / (len(months) - 1)) * chart_w

    def y(v: int) -> float:
        return padding["top"] + chart_h - (v / max_val) * chart_h

    # Build smooth area path.
    points = [(x(i), y(v)) for i, v in enumerate(values)]
    path_d = f"M {points[0][0]:.1f} {points[0][1]:.1f}"
    for i in range(1, len(points)):
        path_d += f" L {points[i][0]:.1f} {points[i][1]:.1f}"
    area_d = f"{path_d} L {points[-1][0]:.1f} {padding['top'] + chart_h:.1f} L {points[0][0]:.1f} {padding['top'] + chart_h:.1f} Z"

    label_x = [m[-2:] for m in months]

    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}" width="100%" role="img" aria-label="Monthly commit activity for {escape_xml(REPO)}">
  <defs>
    <linearGradient id="areaGradient" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="{COLORS['area_top']}" stop-opacity="0.45"/>
      <stop offset="100%" stop-color="{COLORS['area_bottom']}" stop-opacity="0.05"/>
    </linearGradient>
    <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="2.5" result="coloredBlur"/>
      <feMerge>
        <feMergeNode in="coloredBlur"/>
        <feMergeNode in="SourceGraphic"/>
      </feMerge>
    </filter>
  </defs>
  <style>
    .bg {{ fill: {COLORS['bg_dark']}; }}
    .grid {{ stroke: {COLORS['grid']}; stroke-width: 1; }}
    .axis-text {{ fill: {COLORS['text']}; font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace; font-size: 11px; }}
    .line {{ fill: none; stroke: {COLORS['line']}; stroke-width: 2.5; stroke-linecap: round; stroke-linejoin: round; filter: url(#glow); }}
    .area {{ fill: url(#areaGradient); }}
    .point {{ fill: {COLORS['bg_dark']}; stroke: {COLORS['line']}; stroke-width: 2; }}
    .title {{ fill: #c9d1d9; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; font-size: 14px; font-weight: 600; }}
    @media (prefers-color-scheme: light) {{
      .bg {{ fill: {COLORS['bg_light']}; }}
      .grid {{ stroke: {COLORS['grid_light']}; }}
      .axis-text {{ fill: {COLORS['text_light']}; }}
      .point {{ fill: {COLORS['bg_light']}; }}
      .title {{ fill: #24292f; }}
    }}
  </style>
  <rect class="bg" width="{width}" height="{height}" rx="8"/>
  <text x="{padding['left']}" y="24" class="title">Monthly commit activity</text>
'''

    # Horizontal grid lines.
    for i in range(5):
        gy = padding["top"] + (i / 4) * chart_h
        svg += f'  <line class="grid" x1="{padding["left"]}" y1="{gy:.1f}" x2="{width - padding["right"]}" y2="{gy:.1f}" stroke-dasharray="4 4"/>\n'

    # X labels.
    for i, lbl in enumerate(label_x):
        svg += f'  <text x="{x(i):.1f}" y="{height - 14}" text-anchor="middle" class="axis-text">{lbl}</text>\n'

    # Y label (max).
    svg += f'  <text x="{padding['left'] - 10}" y="{padding['top'] + 4}" text-anchor="end" class="axis-text">{max_val}</text>\n'

    # Animated area + line.
    svg += f'  <path class="area" d="{area_d}">\n'
    svg += '    <animate attributeName="opacity" from="0" to="1" dur="1s" fill="freeze"/>\n'
    svg += '  </path>\n'

    path_len = sum(
        ((points[i][0] - points[i - 1][0]) ** 2 + (points[i][1] - points[i - 1][1]) ** 2) ** 0.5
        for i in range(1, len(points))
    )
    svg += f'  <path class="line" d="{path_d}" stroke-dasharray="{path_len:.1f}" stroke-dashoffset="{path_len:.1f}">\n'
    svg += f'    <animate attributeName="stroke-dashoffset" from="{path_len:.1f}" to="0" dur="1.5s" fill="freeze" calcMode="spline" keySplines="0.4 0 0.2 1" keyTimes="0;1"/>\n'
    svg += '  </path>\n'

    # Data points.
    for i, (px, py) in enumerate(points):
        svg += f'  <circle class="point" cx="{px:.1f}" cy="{py:.1f}" r="4" opacity="0">\n'
        svg += f'    <animate attributeName="opacity" from="0" to="1" begin="{0.8 + i * 0.08:.2f}s" dur="0.3s" fill="freeze"/>\n'
        svg += '  </circle>\n'
        # Tooltip-like title on hover.
        svg += f'  <title>{months[i]}: {values[i]} commits</title>\n'

    svg += '</svg>'
    return svg


def build_heatmap(daily: Dict[date, int]) -> str:
    """Build an animated GitHub-style weekly commit heatmap SVG."""
    days, weeks_count, max_count = heatmap_buckets(daily)
    cell_size = 12
    cell_gap = 3
    week_w = cell_size + cell_gap
    day_h = cell_size + cell_gap
    left_pad = 40
    top_pad = 52
    right_pad = 20
    bottom_pad = 30
    width = left_pad + weeks_count * week_w + right_pad
    height = top_pad + 7 * day_h + bottom_pad

    week_days = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]

    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}" width="100%" role="img" aria-label="Weekly commit heatmap for {escape_xml(REPO)}">
  <style>
    .bg {{ fill: {COLORS['bg_dark']}; }}
    .axis-text {{ fill: {COLORS['text']}; font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace; font-size: 10px; }}
    .title {{ fill: #c9d1d9; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; font-size: 14px; font-weight: 600; }}
    .cell {{ width: {cell_size}px; height: {cell_size}px; rx: 2; ry: 2; }}
    .cell:hover {{ stroke: #c9d1d9; stroke-width: 1.5; }}
    @media (prefers-color-scheme: light) {{
      .bg {{ fill: {COLORS['bg_light']}; }}
      .axis-text {{ fill: {COLORS['text_light']}; }}
      .title {{ fill: #24292f; }}
      .cell:hover {{ stroke: #24292f; }}
    }}
  </style>
  <rect class="bg" width="{width}" height="{height}" rx="8"/>
  <text x="{left_pad}" y="24" class="title">Weekly commit heatmap</text>
'''

    # Month labels on top.
    month_label_positions: List[Tuple[str, float]] = []
    prev_month = None
    for idx, (d, _) in enumerate(days):
        if d.weekday() == 0:
            monday_week = idx // 7
            if d.month != prev_month:
                month_label_positions.append((d.strftime("%b"), left_pad + monday_week * week_w))
                prev_month = d.month

    for label, lx in month_label_positions:
        svg += f'  <text x="{lx:.1f}" y="{top_pad - 10}" class="axis-text">{label}</text>\n'

    # Day-of-week labels.
    for i, wd in enumerate(week_days):
        if i % 2 == 0:
            svg += f'  <text x="{left_pad - 8}" y="{top_pad + i * day_h + cell_size}" text-anchor="end" class="axis-text">{wd}</text>\n'

    # Cells.
    for idx, (d, count) in enumerate(days):
        week = idx // 7
        day = idx % 7
        cx = left_pad + week * week_w
        cy = top_pad + day * day_h
        fill = color_for_count(count, max_count, COLORS["heat"])
        delay = (week * 7 + day) * 0.003
        svg += f'  <rect class="cell" x="{cx}" y="{cy}" fill="{fill}">\n'
        svg += f'    <animate attributeName="opacity" from="0" to="1" begin="{delay:.3f}s" dur="0.4s" fill="freeze"/>\n'
        svg += f'    <title>{d.isoformat()}: {count} commit{"s" if count != 1 else ""}</title>\n'
        svg += '  </rect>\n'

    # Legend.
    legend_y = height - 16
    legend_x = width - right_pad - 5 * (cell_size + 6)
    svg += f'  <text x="{legend_x - 6}" y="{legend_y + cell_size - 2}" text-anchor="end" class="axis-text">Less</text>\n'
    for i, fill in enumerate(COLORS["heat"]):
        lx = legend_x + i * (cell_size + 4)
        svg += f'  <rect class="cell" x="{lx}" y="{legend_y}" fill="{fill}"/>\n'
    svg += f'  <text x="{legend_x + len(COLORS["heat"]) * (cell_size + 4) + 4}" y="{legend_y + cell_size - 2}" class="axis-text">More</text>\n'

    svg += '</svg>'
    return svg


def main() -> int:
    since = datetime.now(timezone.utc) - timedelta(days=365)
    print(f"Fetching commits for {REPO} since {since.date().isoformat()}...")
    commits = fetch_commits_since(since)
    print(f"Fetched {len(commits)} commits")

    monthly = monthly_counts(commits)
    daily = daily_counts(commits)

    line_svg = build_line_chart(monthly)
    heatmap_svg = build_heatmap(daily)

    (OUT_DIR / "repo-activity-line.svg").write_text(line_svg, encoding="utf-8")
    (OUT_DIR / "repo-activity-heatmap.svg").write_text(heatmap_svg, encoding="utf-8")

    print("Wrote dist/repo-activity-line.svg")
    print("Wrote dist/repo-activity-heatmap.svg")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
