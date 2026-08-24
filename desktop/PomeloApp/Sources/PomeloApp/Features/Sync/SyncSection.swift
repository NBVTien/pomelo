import SwiftUI

struct SyncSection: View {
    @StateObject private var vm = SyncViewModel()

    var body: some View {
        Section {
            Toggle("Keep main fresh", isOn: $vm.refreshMain)
                .onChange(of: vm.refreshMain) { if vm.loaded { Task { await vm.save() } } }
            if vm.refreshMain {
                Stepper("Every \(vm.intervalMin) min", value: $vm.intervalMin, in: 5...240, step: 5)
                    .onChange(of: vm.intervalMin) { if vm.loaded { Task { await vm.save() } } }
            }
        } header: { Text("Golden source") } footer: {
            Text("Periodically git pull --ff-only + migrate every repo in main so new workspaces clone an up-to-date node_modules and seeded DBs. Runs in the background.")
        }
        .task { await vm.load() }
    }
}
