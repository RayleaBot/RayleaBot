import { AppShellView, type AppShellViewProps } from "./AppShellView";

type AppShellProps = AppShellViewProps;

export type { AppShellProps };

export function AppShell(props: AppShellProps) {
  return <AppShellView {...props} />;
}
