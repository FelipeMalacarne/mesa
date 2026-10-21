/**
 * Small SVG charts in the design system's `.m-chart` look: a sparkline for stat tiles and a
 * multi-series line chart with a crosshair tooltip for the Monitor. Colours come from
 * `--chart-1..5` through the `.s1..s5` classes; nothing here hard-codes a colour.
 */
import * as React from "react";

export interface Series {
  name: string;
  values: number[];
}

function extent(series: Series[]): [number, number] {
  let max = 0;
  for (const s of series) for (const v of s.values) if (Number.isFinite(v) && v > max) max = v;
  return [0, max === 0 ? 1 : niceCeil(max)];
}

function niceCeil(v: number): number {
  const p = Math.pow(10, Math.floor(Math.log10(v)));
  for (const m of [1, 2, 2.5, 5, 10]) if (m * p >= v) return m * p;
  return 10 * p;
}

function path(values: number[], w: number, h: number, max: number, pad = 2): string {
  if (values.length === 0) return "";
  const step = values.length > 1 ? (w - pad * 2) / (values.length - 1) : 0;
  return values
    .map((v, i) => {
      const x = pad + i * step;
      const y = h - pad - (Math.max(0, v) / max) * (h - pad * 2);
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
}

/** A 32px-high trend line for a stat tile. */
export function Sparkline({ values, series = 1 }: { values: number[]; series?: 1 | 2 | 3 | 4 | 5 }) {
  const w = 200;
  const h = 32;
  const [, max] = extent([{ name: "", values }]);
  const d = path(values, w, h, max);
  return (
    <svg className="m-chart m-stat__spark" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" aria-hidden="true">
      {values.length > 1 ? (
        <>
          {series === 1 ? <path className="area" d={`${d} L${w - 2},${h - 2} L2,${h - 2} Z`} /> : null}
          <path className={`line s${series}`} d={d} vectorEffect="non-scaling-stroke" />
        </>
      ) : null}
    </svg>
  );
}

interface LineChartProps {
  series: Series[];
  labels: string[];
  height?: number;
  format?: (v: number) => string;
  ariaLabel: string;
}

/** A line chart with a y grid, the last label on the x axis and a hover crosshair. */
/** Whole numbers stay whole; fractions keep one decimal (so 0.5 does not read as 1). */
function tickText(v: number): string {
  return Number.isInteger(v) ? String(v) : v.toFixed(1);
}

export function LineChart({ series, labels, height = 180, format = tickText, ariaLabel }: LineChartProps) {
  const ref = React.useRef<HTMLDivElement>(null);
  const [width, setWidth] = React.useState(600);
  const [hover, setHover] = React.useState<number | null>(null);
  React.useEffect(() => {
    if (!ref.current) return;
    const ro = new ResizeObserver((entries) => {
      for (const e of entries) setWidth(Math.max(200, Math.floor(e.contentRect.width)));
    });
    ro.observe(ref.current);
    return () => ro.disconnect();
  }, []);
  const left = 40;
  const bottom = 18;
  const w = width - left;
  const h = height - bottom;
  const [, max] = extent(series);
  const n = Math.max(...series.map((s) => s.values.length), 0);
  const step = n > 1 ? w / (n - 1) : 0;
  const ticks = [0, max / 2, max];
  const onMove = (e: React.PointerEvent<SVGSVGElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left - left;
    if (n < 2 || x < 0) return setHover(null);
    setHover(Math.min(n - 1, Math.max(0, Math.round(x / step))));
  };
  return (
    <div ref={ref} style={{ position: "relative" }}>
      <svg className="m-chart" width={width} height={height} role="img" aria-label={ariaLabel} onPointerMove={onMove} onPointerLeave={() => setHover(null)}>
        {ticks.map((t) => {
          const y = h - (t / max) * (h - 4);
          return (
            <g key={t}>
              <line className="grid" x1={left} x2={width} y1={y} y2={y} />
              <text className="tick" x={left - 6} y={y + 4} textAnchor="end">
                {format(t)}
              </text>
            </g>
          );
        })}
        {labels.length > 0 ? (
          <>
            <text className="tick" x={left} y={height - 4}>
              {labels[0]}
            </text>
            <text className="tick" x={width} y={height - 4} textAnchor="end">
              {labels[labels.length - 1]}
            </text>
          </>
        ) : null}
        <g transform={`translate(${left},0)`}>
          {series.map((s, i) => (
            <path key={s.name} className={`line s${(i % 5) + 1}`} d={path(s.values, w, h, max, 0)} />
          ))}
          {hover !== null ? (
            <>
              <line className="xhair" x1={hover * step} x2={hover * step} y1={0} y2={h} />
              {series.map((s, i) => {
                const v = s.values[hover] ?? 0;
                return <circle key={s.name} className={`dot f${(i % 3) + 1}`} cx={hover * step} cy={h - (Math.max(0, v) / max) * h} r={4} />;
              })}
            </>
          ) : null}
        </g>
      </svg>
      {hover !== null ? (
        <div className="m-tooltip" style={{ left: Math.min(width - 160, left + hover * step + 12), top: 8 }}>
          <div className="m-muted">{labels[hover]}</div>
          {series.map((s) => (
            <div key={s.name}>
              {s.name}: <span className="m-mono">{format(s.values[hover] ?? 0)}</span>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
