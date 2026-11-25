import { useState } from "react";
import type { DayLow } from "@/services/api";
import { formatMoney } from "@/utils/money";

const W = 640;
const H = 200;
const PAD = { top: 12, right: 16, bottom: 24, left: 92 }; // left: room for "CN¥12,345.00"
const LINE = "#2a78d6"; // validated against the white page surface
const GRID = "#e5e7eb";

type Props = { history: DayLow[] };

/** Lowest price per day: one series, so no legend; the heading names it. */
export function PriceChart({ history }: Props) {
  const [hover, setHover] = useState<number | null>(null);
  const [table, setTable] = useState(false);
  if (history.length === 0) return <p className="text-slate-600">No price history yet.</p>;
  const first = history[0];
  if (history.length === 1 && first) {
    // One point is not a line: say it instead of drawing an empty plot.
    return (
      <p className="text-slate-600">
        Lowest so far: <strong className="text-slate-900">{formatMoney(first.converted)}</strong> on{" "}
        {first.day}. The chart appears once there are prices from a second day.
      </p>
    );
  }

  const values = history.map((d) => d.converted.minor);
  const lo = Math.min(...values);
  const hi = Math.max(...values);
  const span = hi - lo || Math.max(hi * 0.05, 1); // flat line: give it room
  const x = (i: number) =>
    PAD.left + (history.length === 1 ? 0.5 : i / (history.length - 1)) * (W - PAD.left - PAD.right);
  const y = (v: number) => PAD.top + (1 - (v - lo) / span) * (H - PAD.top - PAD.bottom);
  const path = history
    .map((d, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(d.converted.minor).toFixed(1)}`)
    .join(" ");
  const last = history.length - 1;
  const point = hover ?? last;
  const shown = history[point];

  const onMove = (e: React.PointerEvent<SVGRectElement>) => {
    const box = e.currentTarget.getBoundingClientRect();
    const rel = ((e.clientX - box.left) / box.width) * (W - PAD.left - PAD.right);
    setHover(Math.round((rel / (W - PAD.left - PAD.right)) * last));
  };

  return (
    <div className="space-y-2">
      <div className="flex items-baseline justify-between text-sm">
        <p className="text-slate-600" aria-live="polite">
          {shown && (
            <>
              <strong className="text-slate-900">{formatMoney(shown.converted)}</strong> on {shown.day}
            </>
          )}
        </p>
        <button type="button" onClick={() => setTable((t) => !t)} className="text-slate-600 hover:underline">
          {table ? "Show chart" : "Show table"}
        </button>
      </div>
      {table ? (
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-slate-500">
              <th className="py-1 font-normal">Day</th>
              <th className="py-1 text-right font-normal">Lowest price</th>
            </tr>
          </thead>
          <tbody>
            {history.map((d) => (
              <tr key={d.day} className="border-t border-slate-100">
                <td className="py-1">{d.day}</td>
                <td className="py-1 text-right tabular-nums">{formatMoney(d.converted)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Lowest price per day">
          {[...new Set([hi, lo])].map((v) => (
            <g key={v}>
              <line x1={PAD.left} x2={W - PAD.right} y1={y(v)} y2={y(v)} stroke={GRID} strokeWidth={1} />
              <text x={PAD.left - 8} y={y(v) + 4} textAnchor="end" className="fill-slate-500 text-[11px]">
                {formatMoney({ minor: v, currency: history[0]?.converted.currency ?? "CNY" })}
              </text>
            </g>
          ))}
          {[0, last].map((i) => (
            <text
              key={i}
              x={x(i)}
              y={H - 6}
              textAnchor={i ? "end" : "start"}
              className="fill-slate-500 text-[11px]"
            >
              {history[i]?.day}
            </text>
          ))}
          {hover !== null && (
            <line
              x1={x(hover)}
              x2={x(hover)}
              y1={PAD.top}
              y2={H - PAD.bottom}
              stroke="#94a3b8"
              strokeWidth={1}
            />
          )}
          <path
            d={path}
            fill="none"
            stroke={LINE}
            strokeWidth={2}
            strokeLinejoin="round"
            strokeLinecap="round"
          />
          {shown && (
            <circle
              cx={x(point)}
              cy={y(shown.converted.minor)}
              r={4}
              fill={LINE}
              stroke="#fff"
              strokeWidth={2}
            />
          )}
          <rect
            x={PAD.left}
            y={0}
            width={W - PAD.left - PAD.right}
            height={H}
            fill="transparent"
            onPointerMove={onMove}
            onPointerLeave={() => setHover(null)}
          />
        </svg>
      )}
    </div>
  );
}
