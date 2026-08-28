import SwiftUI

// Decoded verbatim from renderflow.Summary; classification comes from Go (ADR 0001).
struct RenderSummary: Decodable, Equatable {
    var windowS = 10
    var targets: [RenderTarget] = []
    init() {}
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        windowS = try c.decodeIfPresent(Int.self, forKey: .windowS) ?? 10
        targets = try c.decodeIfPresent([RenderTarget].self, forKey: .targets) ?? []
    }
    enum K: String, CodingKey { case windowS = "window_s", targets }
}

struct RenderTarget: Decodable, Equatable, Identifiable {
    var repo = ""; var svc = ""; var commits = 0; var truncated = false; var probeMs = 0.0; var lastSeen: Int64 = 0
    var components: [RenderComponent] = []
    var id: String { repo + "/" + svc }
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        repo = try c.decodeIfPresent(String.self, forKey: .repo) ?? ""
        svc = try c.decodeIfPresent(String.self, forKey: .svc) ?? ""
        commits = try c.decodeIfPresent(Int.self, forKey: .commits) ?? 0
        truncated = try c.decodeIfPresent(Bool.self, forKey: .truncated) ?? false
        probeMs = try c.decodeIfPresent(Double.self, forKey: .probeMs) ?? 0
        lastSeen = try c.decodeIfPresent(Int64.self, forKey: .lastSeen) ?? 0
        components = try c.decodeIfPresent([RenderComponent].self, forKey: .components) ?? []
    }
    enum K: String, CodingKey { case repo, svc, commits, truncated, probeMs = "probe_ms", lastSeen = "last_seen", components }
}

struct RenderComponent: Decodable, Equatable, Identifiable {
    struct Src: Decodable, Equatable { var file = ""; var line = 0 }
    var name = ""; var vendor = false; var renders = 0; var wasted = 0; var selfAvg = 0.0; var selfMax = 0.0
    var why: [String: Int] = [:]; var flags: [String] = []; var src: Src?
    var id: String { name }
    var wastedPct: Int { renders > 0 ? Int((Double(wasted) / Double(renders) * 100).rounded()) : 0 }
    var topWhy: String { why.max { $0.value < $1.value }?.key ?? "" }
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        name = try c.decodeIfPresent(String.self, forKey: .name) ?? ""
        vendor = try c.decodeIfPresent(Bool.self, forKey: .vendor) ?? false
        renders = try c.decodeIfPresent(Int.self, forKey: .renders) ?? 0
        wasted = try c.decodeIfPresent(Int.self, forKey: .wasted) ?? 0
        selfAvg = try c.decodeIfPresent(Double.self, forKey: .selfAvg) ?? 0
        selfMax = try c.decodeIfPresent(Double.self, forKey: .selfMax) ?? 0
        why = try c.decodeIfPresent([String: Int].self, forKey: .why) ?? [:]
        flags = try c.decodeIfPresent([String].self, forKey: .flags) ?? []
        src = try c.decodeIfPresent(Src.self, forKey: .src)
    }
    enum K: String, CodingKey { case name, vendor, renders, wasted, selfAvg = "self_avg", selfMax = "self_max", why, flags, src }
}

struct RenderProbe: Decodable, Equatable, Identifiable {
    var repo = ""; var svc = ""; var target = ""; var react = false; var enabled = false; var source = "auto"
    var id: String { target }
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        repo = try c.decodeIfPresent(String.self, forKey: .repo) ?? ""
        svc = try c.decodeIfPresent(String.self, forKey: .svc) ?? ""
        target = try c.decodeIfPresent(String.self, forKey: .target) ?? ""
        react = try c.decodeIfPresent(Bool.self, forKey: .react) ?? false
        enabled = try c.decodeIfPresent(Bool.self, forKey: .enabled) ?? false
        source = try c.decodeIfPresent(String.self, forKey: .source) ?? "auto"
    }
    enum K: String, CodingKey { case repo, svc, target, react, enabled, source }
}

private struct ProbeList: Decodable {
    var probes: [RenderProbe] = []
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        probes = try c.decodeIfPresent([RenderProbe].self, forKey: .probes) ?? []
    }
    enum K: String, CodingKey { case probes }
}

struct RenderGraphNode: Decodable, Equatable, Identifiable {
    var name = ""; var renders = 0; var wasted = 0; var selfAvg = 0.0; var selfMax = 0.0; var selfSum = 0.0
    var why: [String: Int] = [:]; var flags: [String] = []; var src: RenderComponent.Src?
    var changed = false; var vendor = false; var depth = 0; var lastSeen: Int64 = 0
    var id: String { name }
    var topWhy: String { why.max { $0.value < $1.value }?.key ?? "" }
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        name = try c.decodeIfPresent(String.self, forKey: .name) ?? ""
        renders = try c.decodeIfPresent(Int.self, forKey: .renders) ?? 0
        wasted = try c.decodeIfPresent(Int.self, forKey: .wasted) ?? 0
        selfAvg = try c.decodeIfPresent(Double.self, forKey: .selfAvg) ?? 0
        selfMax = try c.decodeIfPresent(Double.self, forKey: .selfMax) ?? 0
        selfSum = try c.decodeIfPresent(Double.self, forKey: .selfSum) ?? 0
        why = try c.decodeIfPresent([String: Int].self, forKey: .why) ?? [:]
        flags = try c.decodeIfPresent([String].self, forKey: .flags) ?? []
        src = try c.decodeIfPresent(RenderComponent.Src.self, forKey: .src)
        changed = try c.decodeIfPresent(Bool.self, forKey: .changed) ?? false
        vendor = try c.decodeIfPresent(Bool.self, forKey: .vendor) ?? false
        depth = try c.decodeIfPresent(Int.self, forKey: .depth) ?? 0
        lastSeen = try c.decodeIfPresent(Int64.self, forKey: .lastSeen) ?? 0
    }
    enum K: String, CodingKey { case name, renders, wasted, selfAvg = "self_avg", selfMax = "self_max", selfSum = "self_sum", why, flags, src, changed, vendor, depth, lastSeen = "last_seen" }
}

struct RenderGraphEdge: Decodable, Equatable, Identifiable {
    var from = ""; var to = ""; var count = 0
    var id: String { from + "->" + to }
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        from = try c.decodeIfPresent(String.self, forKey: .from) ?? ""
        to = try c.decodeIfPresent(String.self, forKey: .to) ?? ""
        count = try c.decodeIfPresent(Int.self, forKey: .count) ?? 0
    }
    enum K: String, CodingKey { case from, to, count }
}

struct RenderGraph: Decodable, Equatable {
    var repo = ""; var svc = ""; var windowS = 10; var commits = 0; var scoped = false
    var nodes: [RenderGraphNode] = []; var edges: [RenderGraphEdge] = []; var triggers: [String] = []
    init() {}
    init(from d: Decoder) throws {
        let c = try d.container(keyedBy: K.self)
        repo = try c.decodeIfPresent(String.self, forKey: .repo) ?? ""
        svc = try c.decodeIfPresent(String.self, forKey: .svc) ?? ""
        windowS = try c.decodeIfPresent(Int.self, forKey: .windowS) ?? 10
        commits = try c.decodeIfPresent(Int.self, forKey: .commits) ?? 0
        scoped = try c.decodeIfPresent(Bool.self, forKey: .scoped) ?? false
        nodes = try c.decodeIfPresent([RenderGraphNode].self, forKey: .nodes) ?? []
        edges = try c.decodeIfPresent([RenderGraphEdge].self, forKey: .edges) ?? []
        triggers = try c.decodeIfPresent([String].self, forKey: .triggers) ?? []
    }
    enum K: String, CodingKey { case repo, svc, windowS = "window_s", commits, scoped, nodes, edges, triggers }
}

@MainActor
final class RenderFlowViewModel: ObservableObject {
    @Published private(set) var summary = RenderSummary()
    @Published private(set) var probes: [RenderProbe] = []
    @Published private(set) var loaded = false
    @Published var window = 10
    @Published var showVendor = false
    @Published var mode: Mode = .graph
    @Published private(set) var graphs: [String: RenderGraph] = [:]   // target -> graph
    // Names whose render count grew on the last poll; the graph pulses them.
    @Published private(set) var pulsing: Set<String> = []
    enum Mode { case graph, list }

    private let api: PRAPI
    init(api: PRAPI = PomCore.shared) { self.api = api }

    var hasData: Bool { summary.targets.contains { $0.commits > 0 } }
    var probeSeen: Bool { !summary.targets.isEmpty }
    var enabledProbes: [RenderProbe] { probes.filter(\.enabled) }
    func visible(_ t: RenderTarget) -> [RenderComponent] { showVendor ? t.components : t.components.filter { !$0.vendor } }
    func hiddenCount(_ t: RenderTarget) -> Int { t.components.filter(\.vendor).count }

    func load(branch: String) async {
        let d = await api.call { $0.renderSummaryData(branch: branch, window: self.window) }
        if let s = PomJSON.decode(RenderSummary.self, from: d) { summary = s }
        let p = await api.call { $0.renderProbesData(branch: branch) }
        if let l = PomJSON.decode(ProbeList.self, from: p) { probes = l.probes }
        var next: [String: RenderGraph] = [:]
        var pulse: Set<String> = []
        for t in summary.targets where t.commits > 0 {
            let key = t.id
            let g = await api.call { $0.renderGraphData(branch: branch, target: key, window: self.window) }
            guard let graph = PomJSON.decode(RenderGraph.self, from: g) else { continue }
            let prev = graphs[key]
            for n in graph.nodes where (prev?.nodes.first { $0.name == n.name }?.renders ?? 0) < n.renders { pulse.insert(key + "|" + n.name) }
            next[key] = graph
        }
        graphs = next
        pulsing = pulse
        loaded = true
    }

    func setProbe(branch: String, target: String, enabled: Bool) async {
        _ = await api.call { $0.renderSetProbe(branch: branch, target: target, enabled: enabled) }
        await load(branch: branch)
    }

    func clear(branch: String) async {
        _ = await api.call { $0.renderClear(branch: branch) }
        summary = RenderSummary()
    }

    // Live view: poll once a second while the tab is on screen.
    func poll(branch: String) async {
        while !Task.isCancelled {
            await load(branch: branch)
            try? await Task.sleep(for: .seconds(1))
        }
    }
}

struct RenderFlowView: View {
    let workspace: Workspace
    @StateObject private var vm = RenderFlowViewModel()

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider().overlay(Theme.borderSoft)
            if !vm.probes.isEmpty {
                probeBar
                Divider().overlay(Theme.borderSoft)
            }
            if !vm.loaded {
                LoadingView(text: "listening for renders…")
            } else if !vm.hasData {
                empty
            } else {
                if vm.mode == .graph {
                    ScrollView([.horizontal, .vertical]) {
                        VStack(alignment: .leading, spacing: 24) {
                            ForEach(vm.summary.targets.filter { $0.commits > 0 }) { t in
                                if let g = vm.graphs[t.id] {
                                    RenderGraphView(graph: g, pulsing: vm.pulsing, targetKey: t.id)
                                }
                            }
                        }
                        .padding(16)
                    }
                } else {
                    ScrollView {
                        VStack(alignment: .leading, spacing: 18) {
                            ForEach(vm.summary.targets) { t in targetSection(t) }
                        }
                        .padding(16)
                        .readingColumn(1100)
                    }
                }
            }
        }
        .task(id: workspace.id) { await vm.poll(branch: workspace.branch) }
    }

    private var header: some View {
        HStack(spacing: 10) {
            Text("RENDERS").font(.system(size: 10.5, weight: .semibold)).kerning(0.6).foregroundStyle(Theme.muted)
            Text("last \(vm.summary.windowS)s").font(Theme.mono(10.5)).foregroundStyle(Theme.dim)
            Spacer()
            Picker("", selection: $vm.mode) {
                Image(systemName: "point.3.connected.trianglepath.dotted").tag(RenderFlowViewModel.Mode.graph)
                Image(systemName: "list.bullet").tag(RenderFlowViewModel.Mode.list)
            }.pickerStyle(.segmented).frame(width: 70).controlSize(.small).help("Graph / list")
            Toggle("Library", isOn: $vm.showVendor).toggleStyle(.checkbox).controlSize(.small)
                .font(.system(size: 11)).foregroundStyle(Theme.fgMuted)
                .help("Show components from node_modules (minified names you cannot change)")
            Picker("", selection: $vm.window) {
                Text("10s").tag(10); Text("30s").tag(30); Text("2m").tag(120)
            }.pickerStyle(.segmented).frame(width: 150).controlSize(.small)
            Button { Task { await vm.clear(branch: workspace.branch) } } label: {
                Image(systemName: "trash").font(.system(size: 11))
            }.buttonStyle(.plain).foregroundStyle(Theme.fgMuted).help("Clear captured renders")
        }
        .padding(.horizontal, 14).padding(.vertical, 8)
        .background(Theme.bgSoft)
    }

    // One switch per ported service; React ones are on by default (detected in the core).
    private var probeBar: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                Text("PROBE").font(.system(size: 10, weight: .semibold)).kerning(0.6).foregroundStyle(Theme.muted)
                ForEach(vm.probes) { p in
                    Toggle(isOn: Binding(get: { p.enabled },
                                         set: { on in Task { await vm.setProbe(branch: workspace.branch, target: p.target, enabled: on) } })) {
                        HStack(spacing: 4) {
                            Text(p.target).font(Theme.mono(10.5))
                            if p.react { Image(systemName: "atom").font(.system(size: 9)).foregroundStyle(Theme.tool) }
                            if p.source != "auto" { Text(p.source).font(.system(size: 9)).foregroundStyle(Theme.dim) }
                        }
                    }
                    .toggleStyle(.switch).controlSize(.mini)
                    .foregroundStyle(p.enabled ? Theme.fg : Theme.fgMuted)
                    .help(p.react ? "React detected in package.json" : "No react dependency found; you can still force the probe on")
                }
                Spacer()
            }
            .padding(.horizontal, 14).padding(.vertical, 6)
        }
        .background(Theme.bgSoft)
    }

    private var empty: some View {
        let on = vm.enabledProbes
        return VStack(spacing: 14) {
            Image(systemName: "waveform.path.ecg").font(.system(size: 30)).foregroundStyle(Theme.dim)
            VStack(spacing: 6) {
                Text(vm.probeSeen ? "Probe connected, no renders yet"
                     : on.isEmpty ? "No probe enabled" : "Waiting for the app")
                    .font(.system(size: 14, weight: .semibold)).foregroundStyle(Theme.fgMuted)
                Text(vm.probeSeen ? "Interact with the app; commits show up here within a second."
                     : on.isEmpty ? "Turn on the probe for a React service above, or set `render_probe: true` in pom.yml."
                     : "Probe is on for \(on.map(\.target).joined(separator: ", ")). Start the service and open it with its Open button (the Pomelo URL, not 127.0.0.1).")
                    .font(.system(size: 12)).foregroundStyle(Theme.dim).multilineTextAlignment(.center)
                    .frame(maxWidth: 440)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func targetSection(_ t: RenderTarget) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Text("\(t.repo)/\(t.svc)").font(.system(size: 12.5, weight: .semibold)).foregroundStyle(Theme.fg)
                Text("\(t.commits) commits").font(Theme.mono(10.5)).foregroundStyle(Theme.fgMuted)
                if t.truncated { chip("truncated", Theme.warn) }
                Spacer()
                Text("probe \(fmt(t.probeMs)) ms").font(Theme.mono(10)).foregroundStyle(Theme.dim)
            }
            let rows = vm.visible(t)
            if rows.isEmpty {
                Text(t.components.isEmpty ? "No component renders in this window."
                     : "Only library components rendered (\(vm.hiddenCount(t)) hidden).")
                    .font(.system(size: 12)).foregroundStyle(Theme.dim)
            } else {
                table(rows)
                if !vm.showVendor, vm.hiddenCount(t) > 0 {
                    Text("\(vm.hiddenCount(t)) library components hidden").font(.system(size: 10.5)).foregroundStyle(Theme.dim)
                }
            }
        }
    }

    private func table(_ comps: [RenderComponent]) -> some View {
        VStack(spacing: 0) {
            row(name: "COMPONENT", renders: "RENDERS", wasted: "WASTED", avg: "AVG MS", max: "MAX MS", why: "WHY", header: true)
            Divider().overlay(Theme.borderSoft)
            ForEach(comps) { c in
                HStack(spacing: 0) {
                    HStack(spacing: 6) {
                        Text(c.name).font(Theme.mono(11.5)).foregroundStyle(Theme.fg).lineLimit(1)
                        ForEach(c.flags, id: \.self) { f in chip(f, flagColor(f)) }
                    }.frame(maxWidth: .infinity, alignment: .leading)
                    cell("\(c.renders)", 70)
                    cell(c.wasted > 0 ? "\(c.wastedPct)%" : "–", 70, tint: c.wasted * 2 >= c.renders && c.wasted > 0 ? Theme.warn : nil)
                    cell(fmt(c.selfAvg), 70)
                    cell(fmt(c.selfMax), 70, tint: c.flags.contains("slow") ? Theme.danger : nil)
                    Text(c.topWhy).font(Theme.mono(10.5)).foregroundStyle(Theme.fgMuted).frame(width: 80, alignment: .leading).padding(.leading, 14)
                }
                .padding(.horizontal, 10).padding(.vertical, 6)
                Divider().overlay(Theme.borderSoft.opacity(0.5))
            }
        }
        .background(Theme.surface, in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(Theme.borderSoft))
    }

    private func row(name: String, renders: String, wasted: String, avg: String, max: String, why: String, header: Bool) -> some View {
        HStack(spacing: 0) {
            Text(name).frame(maxWidth: .infinity, alignment: .leading)
            Text(renders).frame(width: 70, alignment: .trailing)
            Text(wasted).frame(width: 70, alignment: .trailing)
            Text(avg).frame(width: 70, alignment: .trailing)
            Text(max).frame(width: 70, alignment: .trailing)
            Text(why).frame(width: 80, alignment: .leading).padding(.leading, 14)
        }
        .font(.system(size: 10, weight: .semibold)).kerning(0.5).foregroundStyle(Theme.muted)
        .padding(.horizontal, 10).padding(.vertical, 6)
    }

    private func cell(_ s: String, _ w: CGFloat, tint: Color? = nil) -> some View {
        Text(s).font(Theme.mono(11)).foregroundStyle(tint ?? Theme.fgSoft).frame(width: w, alignment: .trailing)
    }

    private func chip(_ s: String, _ c: Color) -> some View {
        Text(s).font(.system(size: 9.5, weight: .semibold)).foregroundStyle(c)
            .padding(.horizontal, 5).padding(.vertical, 1).background(c.opacity(0.15), in: Capsule())
    }

    private func flagColor(_ f: String) -> Color {
        switch f { case "slow": return Theme.danger; case "wasted": return Theme.warn; default: return Theme.accent }
    }

    private func fmt(_ v: Double) -> String { v >= 10 ? String(Int(v.rounded())) : String(format: "%.1f", v) }
}


// Left-to-right propagation graph: a column per depth, a box per component,
// a curve parent -> child whose weight is how often the child rendered inside it.
struct RenderGraphView: View {
    let graph: RenderGraph
    let pulsing: Set<String>
    let targetKey: String
    @State private var hovered: String?

    private let boxW: CGFloat = 200, boxH: CGFloat = 48, colGap: CGFloat = 70, rowGap: CGFloat = 14

    private struct Placed: Identifiable { let node: RenderGraphNode; let x: CGFloat; let y: CGFloat; var id: String { node.name } }

    private var placed: [Placed] {
        let cols = Dictionary(grouping: graph.nodes, by: \.depth)
        let maxRows = cols.values.map(\.count).max() ?? 0
        let totalH = CGFloat(maxRows) * (boxH + rowGap)
        var out: [Placed] = []
        for (d, ns) in cols.sorted(by: { $0.key < $1.key }) {
            let colH = CGFloat(ns.count) * (boxH + rowGap)
            let y0 = (totalH - colH) / 2
            for (i, n) in ns.enumerated() {
                out.append(Placed(node: n, x: CGFloat(d) * (boxW + colGap), y: y0 + CGFloat(i) * (boxH + rowGap)))
            }
        }
        return out
    }

    private var size: CGSize {
        let depth = (graph.nodes.map(\.depth).max() ?? 0) + 1
        let rows = Dictionary(grouping: graph.nodes, by: \.depth).values.map(\.count).max() ?? 1
        return CGSize(width: CGFloat(depth) * (boxW + colGap) - colGap + 2, height: CGFloat(rows) * (boxH + rowGap))
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 8) {
                Text("\(graph.repo)/\(graph.svc)").font(.system(size: 12.5, weight: .semibold)).foregroundStyle(Theme.fg)
                Text("\(graph.commits) commits").font(Theme.mono(10.5)).foregroundStyle(Theme.fgMuted)
                if graph.scoped {
                    Text("scoped to this branch's changes").font(.system(size: 10.5)).foregroundStyle(Theme.accent)
                        .padding(.horizontal, 6).padding(.vertical, 1).background(Theme.accentSoft, in: Capsule())
                } else {
                    Text("no changed component rendered yet - showing everything").font(.system(size: 10.5)).foregroundStyle(Theme.dim)
                }
                Spacer()
                if let t = graph.triggers.first {
                    Text(t).font(Theme.mono(10)).foregroundStyle(Theme.dim).lineLimit(1)
                }
            }
            if graph.nodes.isEmpty {
                Text("Only library components rendered in this window.").font(.system(size: 12)).foregroundStyle(Theme.dim)
            } else {
                canvas
                legend
            }
        }
    }

    private var canvas: some View {
        let ps = placed
        let pos = Dictionary(uniqueKeysWithValues: ps.map { ($0.node.name, CGPoint(x: $0.x, y: $0.y)) })
        let maxCount = max(1, graph.edges.map(\.count).max() ?? 1)
        return ZStack(alignment: .topLeading) {
            Canvas { ctx, _ in
                for e in graph.edges {
                    guard let a = pos[e.from], let b = pos[e.to] else { continue }
                    let p1 = CGPoint(x: a.x + boxW, y: a.y + boxH / 2)
                    let p2 = CGPoint(x: b.x, y: b.y + boxH / 2)
                    var p = Path()
                    p.move(to: p1)
                    let mx = (p1.x + p2.x) / 2
                    p.addCurve(to: p2, control1: CGPoint(x: mx, y: p1.y), control2: CGPoint(x: mx, y: p2.y))
                    let on = hovered == nil || hovered == e.from || hovered == e.to
                    let w = 1 + 3 * CGFloat(e.count) / CGFloat(maxCount)
                    ctx.stroke(p, with: .color(on ? Theme.fgMuted.opacity(0.7) : Theme.borderSoft.opacity(0.25)), lineWidth: on ? w : 1)
                    if e.count > 1, on {
                        ctx.draw(Text("×\(e.count)").font(Theme.mono(9)).foregroundColor(Theme.dim), at: CGPoint(x: mx, y: (p1.y + p2.y) / 2 - 7))
                    }
                }
            }
            .frame(width: size.width, height: size.height)
            ForEach(ps) { p in
                box(p.node)
                    .position(x: p.x + boxW / 2, y: p.y + boxH / 2)
                    .opacity(hovered == nil || hovered == p.node.name || graph.edges.contains { ($0.from == hovered && $0.to == p.node.name) || ($0.to == hovered && $0.from == p.node.name) } ? 1 : 0.35)
            }
        }
        .frame(width: size.width, height: size.height, alignment: .topLeading)
    }

    private func box(_ n: RenderGraphNode) -> some View {
        let slow = n.flags.contains("slow"), wasted = n.flags.contains("wasted")
        let border: Color = slow ? Theme.danger : wasted ? Theme.warn : n.changed ? Theme.accent : Theme.border
        let pulse = pulsing.contains(targetKey + "|" + n.name)
        return VStack(alignment: .leading, spacing: 3) {
            HStack(spacing: 5) {
                if n.changed { Circle().fill(Theme.accent).frame(width: 6, height: 6) }
                Text(n.name).font(Theme.mono(11.5, .medium)).foregroundStyle(Theme.fg).lineLimit(1).truncationMode(.middle)
                Spacer(minLength: 0)
                if slow { Image(systemName: "tortoise.fill").font(.system(size: 9)).foregroundStyle(Theme.danger) }
            }
            HStack(spacing: 6) {
                Text("\(n.renders)×").font(Theme.mono(10)).foregroundStyle(pulse ? Theme.accent : Theme.fgMuted)
                if n.wasted > 0 { Text("\(n.wasted) wasted").font(.system(size: 10)).foregroundStyle(Theme.warn) }
                Spacer(minLength: 0)
                Text(n.selfMax >= 1 ? String(format: "%.0f ms", n.selfMax) : String(format: "%.1f ms", n.selfMax))
                    .font(Theme.mono(10)).foregroundStyle(slow ? Theme.danger : Theme.dim)
            }
        }
        .padding(.horizontal, 10).padding(.vertical, 7)
        .frame(width: boxW, height: boxH, alignment: .leading)
        .background(Theme.surface, in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(border.opacity(n.changed || slow || wasted ? 0.9 : 0.5), lineWidth: n.changed ? 1.5 : 1))
        .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(Theme.accent, lineWidth: 2).opacity(pulse ? 0.9 : 0))
        .animation(.easeOut(duration: 0.6), value: pulse)
        .opacity(n.changed || !graph.scoped ? 1 : 0.75)
        .contentShape(Rectangle())
        .onHover { hovered = $0 ? n.name : (hovered == n.name ? nil : hovered) }
        .help("\(n.name)\n\(n.renders) renders, \(n.wasted) wasted (\(n.topWhy))\nself avg \(String(format: "%.2f", n.selfAvg)) ms, max \(String(format: "%.2f", n.selfMax)) ms" + (n.src.map { "\n\($0.file):\($0.line)" } ?? ""))
    }

    private var legend: some View {
        HStack(spacing: 14) {
            legendItem(Theme.accent, "changed in this branch")
            legendItem(Theme.warn, "mostly wasted renders")
            legendItem(Theme.danger, "slow (≥16 ms)")
            Text("edge width = how often the child re-rendered inside the parent").font(.system(size: 10)).foregroundStyle(Theme.dim)
        }
    }

    private func legendItem(_ c: Color, _ s: String) -> some View {
        HStack(spacing: 4) {
            RoundedRectangle(cornerRadius: 2).strokeBorder(c, lineWidth: 1.5).frame(width: 12, height: 8)
            Text(s).font(.system(size: 10)).foregroundStyle(Theme.dim)
        }
    }
}
