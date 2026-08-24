import SwiftUI

struct PomeloApp: App {
    @StateObject private var state = AppState()
    @StateObject private var theme = ThemeManager()
    @StateObject private var ui = UIStore()
    @StateObject private var updater = AppUpdater.shared

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(state)
                .environmentObject(theme)
                .environmentObject(ui)
                .frame(minWidth: 1040, minHeight: 640)
                .onAppear {
                    state.uiStore = ui; state.themeManager = theme; state.boot(); theme.applyToWindow()
                    DispatchQueue.main.asyncAfter(deadline: .now() + 1.2) { state.maybeShowSetupOnFirstRun() }
                }
        }
        .windowStyle(.hiddenTitleBar)
        .commands {
            CommandGroup(replacing: .newItem) { }
            CommandGroup(after: .appInfo) {
                Button("Check for Updates…") { updater.checkForUpdates() }
                    .disabled(!updater.canCheckForUpdates)
            }
        }

        Window("Create workspace", id: "create-workspace") {
            CreateWorkspaceView()
                .environmentObject(state)
                .environmentObject(theme)
                .environmentObject(ui)
        }
        .windowResizability(.contentSize)
        .defaultSize(width: 540, height: 720)
        .defaultPosition(.center)

        Window("Settings", id: "settings") {
            SettingsView()
                .environmentObject(state)
                .environmentObject(theme)
                .environmentObject(ui)
        }
        .windowStyle(.hiddenTitleBar)
        .windowResizability(.contentSize)
        .defaultSize(width: 880, height: 600)
        .defaultPosition(.center)
    }
}
