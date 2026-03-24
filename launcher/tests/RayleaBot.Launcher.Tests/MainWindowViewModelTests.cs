using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class MainWindowViewModelTests
{
    [TestMethod]
    public async Task InitializeAsync_UsesChineseStatusCopyAndDefaultsToOverview()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.AreEqual(LauncherSection.Overview, viewModel.ActiveSection);
        Assert.IsTrue(viewModel.IsOverviewSectionActive);
        Assert.AreEqual("检测到现有服务", viewModel.StatusSummary);
        Assert.AreEqual("检测到现有服务", viewModel.HeroTitle);
        CollectionAssert.AreEqual(
            new[] { "总览", "服务控制", "环境检查", "设置", "诊断" },
            viewModel.NavigationItems.Select(item => item.Title).ToArray());
    }

    [TestMethod]
    public async Task InitializeAsync_UsesTitleOnlyNavigationItems()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        CollectionAssert.AreEqual(
            new[] { string.Empty, string.Empty, string.Empty, string.Empty, string.Empty },
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
        Assert.IsFalse(viewModel.IsOverviewSectionActive);
        Assert.IsFalse(viewModel.IsSettingsSectionActive);
    }

    [TestMethod]
    public async Task InitializeAsync_UsesOpenWebLabelEvenWhenSetupIsStillRequired()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.SetupInitialized = false;
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.AreEqual("打开管理界面", viewModel.OpenWebUiActionLabel);
        Assert.AreEqual("检测到现有服务", viewModel.StatusSummary);
    }

    [TestMethod]
    public async Task InitializeAsync_DisablesStartAndEnablesStopWhenExternalServiceIsDetected()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.IsFalse(viewModel.CanStart);
        Assert.IsTrue(viewModel.CanStop);
        Assert.IsTrue(viewModel.CanOpenWebUi);
    }

    [TestMethod]
    public async Task InitializeAsync_UsesExternalServiceCopyWithoutSessionOrSetupWords()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        Assert.AreEqual("端口上已有服务正在运行。可以直接打开管理界面，或先停止它再由启动器重新启动。", viewModel.ServiceDetail);
        Assert.AreEqual(string.Empty, viewModel.SessionSummary);
        Assert.IsFalse(viewModel.ServiceDetail.Contains("初始化", StringComparison.Ordinal));
        Assert.IsFalse(viewModel.ServiceDetail.Contains("会话", StringComparison.Ordinal));
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
