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
        Assert.AreEqual("可用", viewModel.StatusSummary);
        Assert.AreEqual("服务已经可用", viewModel.HeroTitle);
        CollectionAssert.AreEqual(
            new[] { "总览", "服务控制", "环境检查", "设置", "诊断" },
            viewModel.NavigationItems.Select(item => item.Title).ToArray());
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
}
