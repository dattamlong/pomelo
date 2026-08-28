import SwiftUI
import AppKit

// P0 review artifact: an authored narrative plus repo-qualified code anchors. The
// prose links each claim to real code via `pom://code?repo=..&path=..&start=..&end=..`
// so a click peeks that file at the range — the multi-repo take on a guided review.
struct Review: Decodable {
    var exists = false
    var id = ""
    var title = ""
    var doc = ""
    var anchors: [ReviewAnchor] = []
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        exists = try c.decodeIfPresent(Bool.self, forKey: .exists) ?? false
        id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
        title = try c.decodeIfPresent(String.self, forKey: .title) ?? ""
        doc = try c.decodeIfPresent(String.self, forKey: .doc) ?? ""
        anchors = try c.decodeIfPresent([ReviewAnchor].self, forKey: .anchors) ?? []
        if !doc.isEmpty { exists = true }
    }
    enum K: String, CodingKey { case exists, id, title, doc, anchors }
}

struct ReviewAnchor: Decodable, Identifiable {
    var id = ""
    var repo = ""
    var path = ""
    var start = 0
    var end = 0
    var side = "head"
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
        repo = try c.decodeIfPresent(String.self, forKey: .repo) ?? ""
        path = try c.decodeIfPresent(String.self, forKey: .path) ?? ""
        start = try c.decodeIfPresent(Int.self, forKey: .start) ?? 0
        end = try c.decodeIfPresent(Int.self, forKey: .end) ?? 0
        side = try c.decodeIfPresent(String.self, forKey: .side) ?? "head"
    }
    enum K: String, CodingKey { case id, repo, path, start, end, side }
}

enum ReviewStore {
    nonisolated static func get(branch: String, isMain: Bool) -> Data {
        PomCore.shared.reviewGetData(branch: branch, isMain: isMain)
    }
    nonisolated static func peek(branch: String, repo: String, path: String, isMain: Bool) -> Data {
        PomCore.shared.filePeekData(branch: branch, repo: repo, path: path, isMain: isMain)
    }
}

struct CodePeekTarget: Identifiable, Equatable {
    let repo: String, path: String, start: Int, end: Int
    var id: String { repo + "/" + path + ":\(start)-\(end)" }
}

struct ReviewPane: View {
    @EnvironmentObject var theme: ThemeManager
    let workspace: Workspace

    @State private var review: Review?
    @State private var loaded = false
    @State private var peek: CodePeekTarget?

    var body: some View {
        Group {
            if let r = review, r.exists {
                ScrollView {
                    VStack(alignment: .leading, spacing: 10) {
                        if !r.title.isEmpty {
                            Text(r.title).font(.system(size: 18, weight: .semibold)).foregroundStyle(Theme.fg)
                        }
                        MarkdownText(r.doc)
                    }
                    .padding(20).readingColumn(940)
                }
                .environment(\.openURL, OpenURLAction { url in handleLink(url) })
            } else if !loaded {
                LoadingView(text: "loading review…")
            } else {
                EmptyStateView(icon: "doc.text.magnifyingglass", title: "No review yet",
                               subtitle: "Ask the agent to author one for this workspace.")
            }
        }
        .background(Theme.bg)
        .task(id: workspace.id) { await load() }
        .sheet(item: $peek) { t in
            CodePeekSheet(target: t, branch: workspace.branch, isMain: workspace.isMain)
                .environmentObject(theme)
        }
    }

    private func handleLink(_ url: URL) -> OpenURLAction.Result {
        guard url.scheme == "pom", url.host == "code",
              let comps = URLComponents(url: url, resolvingAgainstBaseURL: false) else {
            NSWorkspace.shared.open(url)
            return .handled
        }
        let q = Dictionary(comps.queryItems?.map { ($0.name, $0.value ?? "") } ?? [], uniquingKeysWith: { a, _ in a })
        guard let repo = q["repo"], let path = q["path"], !repo.isEmpty, !path.isEmpty else { return .handled }
        peek = CodePeekTarget(repo: repo, path: path, start: Int(q["start"] ?? "") ?? 0, end: Int(q["end"] ?? "") ?? 0)
        return .handled
    }

    private func load() async {
        let branch = workspace.branch, isMain = workspace.isMain
        let r = await Task.detached(priority: .userInitiated) { () -> Review? in
            PomJSON.decode(Review.self, from: ReviewStore.get(branch: branch, isMain: isMain))
        }.value
        review = r
        loaded = true
    }
}

// Peeks a file at a line range without leaving the review.
struct CodePeekSheet: View {
    @EnvironmentObject var theme: ThemeManager
    @Environment(\.dismiss) private var dismiss
    let target: CodePeekTarget
    let branch: String
    let isMain: Bool

    @State private var lines: [String]?

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 8) {
                Image(systemName: "doc.text").font(.system(size: 11)).foregroundStyle(Theme.accent)
                Text(target.repo).font(Theme.mono(11, .semibold)).foregroundStyle(Theme.accent)
                Text(target.path).font(Theme.mono(11)).foregroundStyle(Theme.fg).lineLimit(1).truncationMode(.middle)
                if target.start > 0 { Text(":\(target.start)-\(target.end)").font(Theme.mono(10.5)).foregroundStyle(Theme.dim) }
                Spacer()
                IconButton("xmark", size: 12) { dismiss() }
            }
            .padding(.horizontal, 14).padding(.vertical, 10)
            Divider().overlay(Theme.borderSoft)
            if let lines {
                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 0) {
                            ForEach(Array(lines.enumerated()), id: \.offset) { i, line in
                                lineRow(n: i + 1, text: line)
                                    .id(i + 1)
                            }
                        }.padding(.vertical, 6)
                    }
                    .onAppear { if target.start > 1 { proxy.scrollTo(max(1, target.start - 2), anchor: .top) } }
                }
            } else {
                LoadingView(text: "loading file…")
            }
        }
        .frame(width: 820, height: 560)
        .background(Theme.bg)
        .task { await load() }
    }

    private func lineRow(n: Int, text: String) -> some View {
        let hit = n >= target.start && n <= max(target.start, target.end)
        return HStack(spacing: 0) {
            Text("\(n)").font(Theme.mono(10.5)).foregroundStyle(Theme.dim)
                .frame(width: 48, alignment: .trailing).padding(.trailing, 10)
            Text(text.isEmpty ? " " : text).font(Theme.mono(11.5)).foregroundStyle(Theme.fgSoft)
                .textSelection(.enabled).fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
            Spacer(minLength: 0)
        }
        .padding(.vertical, 1)
        .background(hit ? Theme.accent.opacity(0.14) : .clear)
    }

    private func load() async {
        let t = target, branch = branch, isMain = isMain
        let ls = await Task.detached(priority: .userInitiated) { () -> [String]? in
            struct R: Decodable { var content = ""; var error = "" }
            guard let r = PomJSON.decode(R.self, from: ReviewStore.peek(branch: branch, repo: t.repo, path: t.path, isMain: isMain)),
                  r.error.isEmpty else { return nil }
            return r.content.components(separatedBy: "\n")
        }.value
        lines = ls ?? ["(file not found)"]
    }
}
