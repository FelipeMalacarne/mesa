/** A read-only result table in the grid look, for query results. */
import type { QueryResult } from "@/lib/api/mesaAPI.schemas";
import { cn } from "@/lib/utils";
import { isNull, isNumeric, textOf } from "@/lib/values";

export function ResultGrid({ result }: { result: QueryResult }) {
  return (
    <table className="m-grid">
      <thead>
        <tr>
          <th className="m-gutter">#</th>
          {result.columns.map((c, i) => (
            <th key={`${c.name}${i}`} className={cn(isNumeric(c.kind) && "is-num")}>
              <span className="m-th">
                <span>{c.name}</span>
                <span className="m-th__type">{c.type}</span>
              </span>
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {result.rows.map((row, r) => (
          <tr key={r}>
            <td className="m-gutter">{r + 1}</td>
            {row.map((v, i) => (
              <td key={i} className={cn(isNumeric(result.columns[i]?.kind) && "is-num")} title={isNull(v) ? undefined : textOf(v).slice(0, 500)}>
                {isNull(v) ? <span className="m-null">NULL</span> : result.columns[i]?.kind === "json" ? <span className="m-json">{textOf(v)}</span> : textOf(v)}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
