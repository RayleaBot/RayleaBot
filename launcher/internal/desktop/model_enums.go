package desktop

type LauncherCloseBehavior string

const (
	CloseAskEveryTime    LauncherCloseBehavior = "ask_every_time"
	CloseHideToTray      LauncherCloseBehavior = "hide_to_tray"
	CloseExitApplication LauncherCloseBehavior = "exit_application"
)

type LauncherCloseAction string

const (
	CloseActionHide   LauncherCloseAction = "hide"
	CloseActionExit   LauncherCloseAction = "exit"
	CloseActionCancel LauncherCloseAction = "cancel"
)

type LauncherProcessLifecycle string

const (
	Stopped  LauncherProcessLifecycle = "stopped"
	Starting LauncherProcessLifecycle = "starting"
	Running  LauncherProcessLifecycle = "running"
	Stopping LauncherProcessLifecycle = "stopping"
)

type LauncherProcessOwnership string

const (
	OwnershipNone     LauncherProcessOwnership = "none"
	OwnershipLauncher LauncherProcessOwnership = "launcher_managed"
	OwnershipExternal LauncherProcessOwnership = "external"
)

type CheckSeverity string

const (
	CheckOK      CheckSeverity = "ok"
	CheckWarning CheckSeverity = "warning"
	CheckError   CheckSeverity = "error"
)

type EnvironmentCheckScope string

const (
	ScopePreflight EnvironmentCheckScope = "preflight"
	ScopeAdvisory  EnvironmentCheckScope = "advisory"
)

type ReleaseCheckStatus string

const (
	ReleaseDisabled        ReleaseCheckStatus = "disabled"
	ReleaseIdle            ReleaseCheckStatus = "idle"
	ReleaseChecking        ReleaseCheckStatus = "checking"
	ReleaseUpToDate        ReleaseCheckStatus = "up_to_date"
	ReleaseUpdateAvailable ReleaseCheckStatus = "update_available"
	ReleaseDownloading     ReleaseCheckStatus = "downloading"
	ReleaseReadyToInstall  ReleaseCheckStatus = "ready_to_install"
	ReleaseInstalling      ReleaseCheckStatus = "installing"
	ReleaseSucceeded       ReleaseCheckStatus = "succeeded"
	ReleaseFailed          ReleaseCheckStatus = "failed"
	ReleaseRolledBack      ReleaseCheckStatus = "rolled_back"
	ReleaseRollbackFailed  ReleaseCheckStatus = "rollback_failed"
)

type RuntimePrepareStatus string

const (
	PreparePending   RuntimePrepareStatus = "pending"
	PrepareRunning   RuntimePrepareStatus = "running"
	PrepareSucceeded RuntimePrepareStatus = "succeeded"
	PrepareFailed    RuntimePrepareStatus = "failed"
)
