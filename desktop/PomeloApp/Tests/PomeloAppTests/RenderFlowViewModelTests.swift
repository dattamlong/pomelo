import XCTest
@testable import PomeloApp

@MainActor
final class RenderFlowViewModelTests: XCTestCase {
    func testDecodesSummaryAndDerivedFields() async {
        let mock = MockPomAPI()
        mock.renderSummaryJSON = #"""
        {"window_s":10,"targets":[{"repo":"client","svc":"portal","commits":3,"truncated":false,"probe_ms":1.5,"last_seen":1,
          "components":[{"name":"LeadTable","renders":4,"wasted":3,"self_avg":11,"self_max":20,"why":{"parent":3,"state":1},"flags":["slow","wasted"]},
                        {"name":"te$1","vendor":true,"renders":9,"wasted":9,"self_avg":0,"self_max":0,"why":{"parent":9},"flags":["wasted"]}]}]}
        """#
        let vm = RenderFlowViewModel(api: mock)
        await vm.load(branch: "feat")
        XCTAssertTrue(vm.loaded)
        XCTAssertTrue(vm.hasData)
        let c = vm.summary.targets[0].components[0]
        XCTAssertEqual(c.wastedPct, 75)
        XCTAssertEqual(c.topWhy, "parent")
        XCTAssertEqual(c.flags, ["slow", "wasted"])
        XCTAssertNil(c.src)
        let t = vm.summary.targets[0]
        XCTAssertEqual(vm.visible(t).map(\.name), ["LeadTable"], "vendor hidden by default")
        XCTAssertEqual(vm.hiddenCount(t), 1)
        vm.showVendor = true
        XCTAssertEqual(vm.visible(t).count, 2)
    }

    func testEmptyTargetsMeansNoProbe() async {
        let vm = RenderFlowViewModel(api: MockPomAPI())
        await vm.load(branch: "feat")
        XCTAssertFalse(vm.probeSeen)
        XCTAssertFalse(vm.hasData)
        XCTAssertTrue(vm.probes.isEmpty)
    }

    func testProbesDecodeAndToggleRoundTrips() async {
        let mock = MockPomAPI()
        mock.renderProbesJSON = #"{"probes":[{"repo":"client","svc":"portal","target":"client/portal","react":true,"enabled":true,"source":"auto"},{"repo":"api","svc":"web","target":"api/web","react":false,"enabled":false,"source":"auto"}]}"#
        let vm = RenderFlowViewModel(api: mock)
        await vm.load(branch: "feat")
        XCTAssertEqual(vm.probes.count, 2)
        XCTAssertEqual(vm.enabledProbes.map(\.target), ["client/portal"])
        await vm.setProbe(branch: "feat", target: "api/web", enabled: true)
        XCTAssertEqual(mock.renderSetProbeCalls.count, 1)
        XCTAssertEqual(mock.renderSetProbeCalls[0].0, "api/web")
        XCTAssertTrue(mock.renderSetProbeCalls[0].1)
    }

    func testClearResetsAndCallsCore() async {
        let mock = MockPomAPI()
        let vm = RenderFlowViewModel(api: mock)
        await vm.clear(branch: "feat")
        XCTAssertEqual(mock.renderClearCalls, 1)
        XCTAssertEqual(vm.summary.targets.count, 0)
    }
}
