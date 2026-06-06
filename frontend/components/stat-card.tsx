import { Card, CardContent } from "@/components/ui/card";

// StatCard is a compact metric tile: a mono uppercase eyebrow label over a
// large value, with an optional sub-line. Used on the dashboard overview row.
export function StatCard({
  label,
  value,
  sub,
}: {
  label: string;
  value: React.ReactNode;
  sub?: string;
}) {
  return (
    <Card className="rounded-lg">
      <CardContent>
        <p className="font-mono text-xs tracking-wider text-muted-foreground uppercase">{label}</p>
        <p className="mt-2 text-2xl font-normal tracking-tight tabular-nums">{value}</p>
        {sub && <p className="mt-1 text-xs text-muted-foreground">{sub}</p>}
      </CardContent>
    </Card>
  );
}
