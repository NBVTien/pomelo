import SwiftUI

struct NMStoreSection: View {
    @StateObject private var vm = NMStoreViewModel()

    var body: some View {
        Section {
            if vm.loading {
                HStack(spacing: 8) { ProgressView().controlSize(.small); Text("scanning…").foregroundStyle(Theme.fgMuted) }
            } else if vm.entries.isEmpty {
                Text("No cached node_modules yet.").font(.system(size: 12)).foregroundStyle(Theme.fgMuted)
            } else {
                ForEach(vm.sorted) { e in row(e) }
                if !vm.stale.isEmpty {
                    Button(role: .destructive) { Task { await vm.deleteStale() } } label: {
                        Label("Delete \(vm.stale.count) stale (\(vm.human(vm.staleBytes)))", systemImage: "trash")
                            .font(.system(size: 12))
                    }
                }
            }
        } header: { Text("Dependency store") } footer: {
            Text("Cached node_modules cloned into new workspaces instead of a full install\(vm.total > 0 ? " · \(vm.human(vm.total)) total" : ""). \"current\" matches main's lockfile; older hashes are safe to delete — the next workspace reinstalls and re-caches.")
        }
        .task { await vm.load() }
    }

    private func row(_ e: NMStoreViewModel.Entry) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 1) {
                HStack(spacing: 6) {
                    Text(e.repo).font(.system(size: 12.5, weight: .medium)).foregroundStyle(Theme.fg)
                    if e.current {
                        Text("current").font(.system(size: 9, weight: .medium)).foregroundStyle(Theme.ok)
                            .padding(.horizontal, 5).padding(.vertical, 1).background(Theme.ok.opacity(0.16), in: Capsule())
                    }
                }
                Text(e.hash).font(Theme.mono(10)).foregroundStyle(Theme.dim)
            }
            Spacer()
            Text(vm.human(e.bytes)).font(Theme.mono(11)).foregroundStyle(Theme.fgMuted)
            Button(role: .destructive) { Task { await vm.delete(e) } } label: { Image(systemName: "trash").font(.system(size: 10)) }
                .buttonStyle(.plain).foregroundStyle(e.current ? Theme.dim : Theme.danger)
        }
    }
}
