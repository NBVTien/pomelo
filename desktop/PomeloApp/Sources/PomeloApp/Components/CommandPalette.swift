import SwiftUI

struct CommandPalette: View {
    @EnvironmentObject var state: AppState
    @Binding var show: Bool
    @State private var query = ""
    @State private var index = 0
    @FocusState private var focused: Bool

    private var results: [Workspace] {
        var seen = Set<String>()
        let all = (state.mainWorkspaces + state.orderedNonMain).filter { seen.insert($0.id).inserted }

        let q = query.trimmingCharacters(in: .whitespaces)
        guard !q.isEmpty else { return all }
        let scored: [(ws: Workspace, score: Int)] = all.compactMap { w in
            let s = [Fuzzy.score(q, w.branch), Fuzzy.score(q, w.title)].compactMap { $0 }.max()
            return s.map { (w, $0) }
        }
        return scored.sorted {
            $0.score != $1.score ? $0.score > $1.score
                : ($0.ws.branch.count != $1.ws.branch.count ? $0.ws.branch.count < $1.ws.branch.count
                   : $0.ws.branch < $1.ws.branch)
        }.map(\.ws)
    }

    var body: some View {
        ZStack(alignment: .top) {
            Color.black.opacity(0.45).ignoresSafeArea().onTapGesture { show = false }

            VStack(spacing: 0) {
                HStack(spacing: 8) {
                    Image(systemName: "magnifyingglass").font(.system(size: 13)).foregroundStyle(Theme.muted)
                    TextField("Switch workspace…", text: $query)
                        .textFieldStyle(.plain).font(.system(size: 15))
                        .focused($focused)
                        .onSubmit(choose)
                    Text("esc").font(Theme.mono(10)).foregroundStyle(Theme.dim)
                        .padding(.horizontal, 5).padding(.vertical, 1)
                        .background(Theme.chip, in: RoundedRectangle(cornerRadius: 4))
                }
                .padding(.horizontal, 14).padding(.vertical, 12)
                Divider().overlay(Theme.borderSoft)

                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(spacing: 1) {
                            ForEach(Array(results.enumerated()), id: \.element.id) { i, ws in
                                row(ws, active: i == index).id(ws.id)
                                    .onTapGesture { index = i; choose() }
                            }
                            if results.isEmpty {
                                Text("no match").font(.system(size: 12)).foregroundStyle(Theme.dim).padding(16)
                            }
                        }
                        .padding(6)
                    }
                    .frame(maxHeight: 340)
                    .onChange(of: index) { _, i in
                        guard i >= 0, i < results.count else { return }
                        withAnimation(.easeOut(duration: 0.1)) { proxy.scrollTo(results[i].id, anchor: .center) }
                    }
                }
            }
            .frame(width: 540)
            .background(Theme.bgSoft, in: RoundedRectangle(cornerRadius: 12))
            .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(Theme.border))
            .shadow(color: .black.opacity(0.5), radius: 30, y: 12)
            .padding(.top, 120)
        }
        .onAppear {
            index = 0
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.05) { focused = true }
        }
        .onChange(of: query) { _, _ in index = 0 }
        .onKeyPress(.downArrow) { index = min(index + 1, max(0, results.count - 1)); return .handled }
        .onKeyPress(.upArrow) { index = max(index - 1, 0); return .handled }
        .onKeyPress(.escape) { show = false; return .handled }
    }

    private func row(_ ws: Workspace, active: Bool) -> some View {
        HStack(spacing: 10) {
            Image(systemName: ws.isMain ? "house.fill" : "arrow.triangle.branch")
                .font(.system(size: 12)).foregroundStyle(active ? Theme.accent : Theme.fgMuted).frame(width: 16)
            Text(ws.title).font(.system(size: 13)).foregroundStyle(Theme.fg).lineLimit(1)
            Spacer()
            Text("\(ws.running)/\(ws.total)").font(Theme.mono(10.5)).foregroundStyle(Theme.dim)
        }
        .padding(.horizontal, 10).padding(.vertical, 7)
        .background(active ? Theme.sel : .clear, in: RoundedRectangle(cornerRadius: 7))
        .contentShape(Rectangle())
    }

    private func choose() {
        guard index < results.count else { return }
        state.selection = results[index].id
        show = false
    }
}
