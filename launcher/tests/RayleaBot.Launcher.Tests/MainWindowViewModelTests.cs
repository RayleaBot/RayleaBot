using RayleaBot.Launcher.Models;
using RayleaBot.Launcher.Services;

namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class MainWindowViewModelTests
{
    [TestMethod]
    public async Task InitializeAsync_DefaultsToStatusNavigationWithFourPages()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.AreEqual(LauncherSection.Status, viewModel.ActiveSection);
        Assert.IsTrue(viewModel.IsStatusSectionActive);
        CollectionAssert.AreEqual(
            new[] { "状态", "环境检查", "日志与诊断", "设置" },
            viewModel.NavigationItems.Select(item => item.Title).ToArray());
        CollectionAssert.AreEqual(
            new[] { false, false, false, true },
            viewModel.NavigationItems.Select(item => item.IsFooterItem).ToArray());
    }

    [TestMethod]
    public async Task InitializeAsync_UsesTitleOnlyNavigationItems()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        CollectionAssert.AreEqual(
            new[] { string.Empty, string.Empty, string.Empty, string.Empty },
            viewModel.NavigationItems.Select(item => item.Summary).ToArray());
    }

    [TestMethod]
    public void SetActiveSection_SwitchesSectionFlags()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        viewModel.SetActiveSection(LauncherSection.Diagnostics);

        Assert.AreEqual(LauncherSection.Diagnostics, viewModel.ActiveSection);
        Assert.IsTrue(viewModel.IsDiagnosticsSectionActive);
        Assert.IsFalse(viewModel.IsStatusSectionActive);
        Assert.IsFalse(viewModel.IsSettingsSectionActive);
    }

    [TestMethod]
    public async Task InitializeAsync_UsesOpenWebAsPrimaryActionWhenServiceIsAlreadyReachable()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.AreEqual("打开管理界面", viewModel.PrimaryActionLabel);
        Assert.AreEqual(LauncherPrimaryAction.OpenWebUi, viewModel.PrimaryAction);
        Assert.IsTrue(viewModel.CanRunPrimaryAction);
        Assert.IsTrue(viewModel.CanStop);
    }

    [TestMethod]
    public async Task InitializeAsync_UsesStartAsPrimaryActionWhenServiceIsStoppedWithoutBlockingIssues()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.HealthDefault = false;
        fixture.EnvironmentInspector.Inspection = new EnvironmentInspection(
        [
            new EnvironmentCheckResult("server.executable", "服务端可执行文件", CheckSeverity.Ok, "已找到可执行文件。", "ok", string.Empty),
            new EnvironmentCheckResult("config.file", "用户配置", CheckSeverity.Ok, "配置文件可读。", "ok", string.Empty),
        ],
        false,
        false);
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.AreEqual("启动服务", viewModel.PrimaryActionLabel);
        Assert.AreEqual(LauncherPrimaryAction.StartService, viewModel.PrimaryAction);
        Assert.IsTrue(viewModel.CanRunPrimaryAction);
    }

    [TestMethod]
    public async Task RunPrimaryActionAsync_ShowsImmediateBusyFeedbackWhileStartIsInFlight()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.HealthDefault = false;
        fixture.EnvironmentInspector.Inspection = new EnvironmentInspection(
        [
            new EnvironmentCheckResult("server.executable", "服务端可执行文件", CheckSeverity.Ok, "已找到可执行文件。", "ok", string.Empty),
            new EnvironmentCheckResult("config.file", "用户配置", CheckSeverity.Ok, "配置文件可读。", "ok", string.Empty),
        ],
        false,
        false);
        var coordinator = fixture.CreateCoordinator(new LauncherCoordinatorOptions(TimeSpan.FromMilliseconds(40), TimeSpan.FromMilliseconds(5), TimeSpan.FromMilliseconds(20)));
        var viewModel = new MainWindowViewModel(coordinator, marshalToUiThread: false);

        await viewModel.InitializeAsync();
        Assert.AreEqual(LauncherPrimaryAction.StartService, viewModel.PrimaryAction);
        var inspectGate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        fixture.EnvironmentInspector.InspectGate = inspectGate;

        var runTask = viewModel.RunPrimaryActionAsync();

        Assert.IsTrue(viewModel.IsActionInProgress);
        Assert.AreEqual("正在启动服务...", viewModel.PendingActionMessage);
        Assert.AreEqual("启动中...", viewModel.PrimaryActionDisplayLabel);
        Assert.AreEqual("正在启动服务...", viewModel.OperationSummary);
        Assert.IsFalse(viewModel.CanRunPrimaryAction);

        fixture.ManagementClient.HealthDefault = true;
        inspectGate.SetResult();
        await runTask;

        Assert.IsFalse(viewModel.IsActionInProgress);
        Assert.AreEqual(string.Empty, viewModel.PendingActionMessage);
    }

    [TestMethod]
    public async Task InitializeAsync_DoesNotPromoteBuildInfoWarningToHomeAlert()
    {
        var fixture = new LauncherFixture();
        fixture.ReleaseFeedClient.Snapshot = ReleaseCheckSnapshot.Unavailable("启动器可执行文件旁缺少 build_info.json。");
        fixture.EnvironmentInspector.Inspection = new EnvironmentInspection(
        [
            new EnvironmentCheckResult("server.executable", "服务端可执行文件", CheckSeverity.Ok, "已找到可执行文件。", "ok", string.Empty),
            new EnvironmentCheckResult("config.file", "用户配置", CheckSeverity.Ok, "配置文件可读。", "ok", string.Empty),
        ],
        false,
        false);
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.IsFalse(viewModel.HasHomeAlert);
        Assert.AreEqual(string.Empty, viewModel.HomeAlertTitle);
    }

    [TestMethod]
    public async Task InitializeAsync_SurfacesTemplateWarningAsHomeAlert()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.HealthDefault = false;
        fixture.EnvironmentInspector.Inspection = new EnvironmentInspection(
        [
            new EnvironmentCheckResult(
                "render.templates_missing",
                "模板资源",
                CheckSeverity.Warning,
                "模板资源缺失。",
                @"缺少模板目录：C:\RayleaBot\templates",
                "启用 render.image 预览链路之前，请先补齐打包模板资源。"),
        ],
        false,
        false);
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.IsTrue(viewModel.HasHomeAlert);
        Assert.AreEqual("模板资源", viewModel.HomeAlertTitle);
        Assert.AreEqual("模板资源缺失。", viewModel.HomeAlertMessage);
    }

    [TestMethod]
    public async Task InitializeAsync_KeepsSettingsReadOnlyUntilEditingBegins()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.IsFalse(viewModel.IsSettingsEditing);
        Assert.IsTrue(viewModel.AreSettingsReadOnly);

        viewModel.BeginSettingsEditing();

        Assert.IsTrue(viewModel.IsSettingsEditing);
        Assert.IsFalse(viewModel.AreSettingsReadOnly);

        viewModel.CancelSettingsEditing();

        Assert.IsFalse(viewModel.IsSettingsEditing);
        Assert.IsTrue(viewModel.AreSettingsReadOnly);
    }

    [TestMethod]
    public void SetWindowState_TracksMaximizedStateWithoutGlyphStrings()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        Assert.IsFalse(viewModel.IsWindowMaximized);

        viewModel.SetWindowState(true);

        Assert.IsTrue(viewModel.IsWindowMaximized);

        viewModel.SetWindowState(false);

        Assert.IsFalse(viewModel.IsWindowMaximized);
    }
}
