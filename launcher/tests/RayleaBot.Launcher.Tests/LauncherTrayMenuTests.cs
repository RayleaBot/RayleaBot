using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class LauncherTrayMenuTests
{
    [TestMethod]
    public async Task BuildEntries_WhenServiceIsStopped_UsesExpectedOrderingAndStates()
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

        var trayMenu = new LauncherTrayMenu(viewModel, _ => { });

        CollectionAssert.AreEqual(
            new[]
            {
                "RayleaBot 启动器|False|",
                "状态：未启动|False|",
                "<separator>",
                "恢复窗口|True|Restore",
                "打开管理界面|False|OpenWeb",
                "启动服务|True|Start",
                "<separator>",
                "打开日志目录|True|OpenLogs",
                "<separator>",
                "完全退出|True|Exit",
            },
            DescribeEntries(trayMenu.BuildEntries()).ToArray());
    }

    [TestMethod]
    public async Task BuildEntries_WhenServiceIsRunning_UsesExpectedOrderingAndStates()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        var trayMenu = new LauncherTrayMenu(viewModel, _ => { });

        CollectionAssert.AreEqual(
            new[]
            {
                "RayleaBot 启动器|False|",
                "状态：运行中|False|",
                "<separator>",
                "恢复窗口|True|Restore",
                "打开管理界面|True|OpenWeb",
                "停止服务|True|Stop",
                "<separator>",
                "打开日志目录|True|OpenLogs",
                "<separator>",
                "完全退出|True|Exit",
            },
            DescribeEntries(trayMenu.BuildEntries()).ToArray());
    }

    [TestMethod]
    public async Task BuildEntries_WhenServiceHasFailed_LeavesStopAvailableAndStartDisabled()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.HealthDefault = false;
        fixture.ProcessController.IsRunningValue = true;
        fixture.EnvironmentInspector.Inspection = new EnvironmentInspection(
        [
            new EnvironmentCheckResult("server.executable", "服务端可执行文件", CheckSeverity.Ok, "已找到可执行文件。", "ok", string.Empty),
            new EnvironmentCheckResult("config.file", "用户配置", CheckSeverity.Ok, "配置文件可读。", "ok", string.Empty),
        ],
        false,
        false);
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        var trayMenu = new LauncherTrayMenu(viewModel, _ => { });

        CollectionAssert.AreEqual(
            new[]
            {
                "RayleaBot 启动器|False|",
                "状态：启动失败|False|",
                "<separator>",
                "恢复窗口|True|Restore",
                "打开管理界面|False|OpenWeb",
                "停止服务|True|Stop",
                "<separator>",
                "打开日志目录|True|OpenLogs",
                "<separator>",
                "完全退出|True|Exit",
            },
            DescribeEntries(trayMenu.BuildEntries()).ToArray());
    }

    [TestMethod]
    public async Task ShouldHandleTrayClick_ReturnsFalseWhileMenuIsOpen()
    {
        var fixture = new LauncherFixture();
        var viewModel = new MainWindowViewModel(fixture.CreateCoordinator(), marshalToUiThread: false);

        await viewModel.InitializeAsync();

        var trayMenu = new LauncherTrayMenu(viewModel, _ => { });

        Assert.IsTrue(trayMenu.ShouldHandleTrayClick());

        trayMenu.OnMenuOpening();

        Assert.IsFalse(trayMenu.ShouldHandleTrayClick());

        trayMenu.OnMenuClosed();

        Assert.IsTrue(trayMenu.ShouldHandleTrayClick());
    }

    private static IEnumerable<string> DescribeEntries(IReadOnlyList<LauncherTrayMenuEntry> entries)
    {
        foreach (var entry in entries)
        {
            if (entry.IsSeparator)
            {
                yield return "<separator>";
                continue;
            }

            yield return $"{entry.Header}|{entry.IsEnabled}|{entry.Action?.ToString() ?? string.Empty}";
        }
    }
}
