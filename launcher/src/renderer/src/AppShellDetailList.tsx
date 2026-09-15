import type { ReactNode } from "react";

type DetailRowProps = {
  icon: ReactNode;
  label: string;
  value?: string;
  title?: string;
  mono?: boolean;
  children?: ReactNode;
};

export function DetailRow({ icon, label, value, title, mono = true, children }: DetailRowProps) {
  const Value = mono ? "code" : "span";
  return (
    <div className="detail-list__row">
      <dt>
        <span className="detail-list__icon" aria-hidden="true">{icon}</span>
        {label}
      </dt>
      <dd>
        {children ?? <Value className="detail-list__value" title={title}>{value}</Value>}
      </dd>
    </div>
  );
}
