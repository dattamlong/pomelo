import XCTest
@testable import PomeloApp

@MainActor
final class RenderFlowViewModelTests: XCTestCase {
    func testDecodesSummaryAndDerivedFields() async {
        let mock = MockPomAPI()
        mock.renderSummaryJSON = #"""
        {"window_s":10,"targets":[{"repo":"client","svc":"portal","commits":3,"truncated":false,"probe_ms":1.5,"last_seen":1,
          "components":[{"name":"LeadTable","renders":4,"wasted":3,"self_avg":11,"self_max":20,"why":{"parent":3,"state":1},"flags":["slow","wasted"]}]}]}
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
    }

    func testEmptyTargetsMeansNoProbe() async {
        let vm = RenderFlowViewModel(api: MockPomAPI())
        await vm.load(branch: "feat")
        XCTAssertFalse(vm.probeSeen)
        XCTAssertFalse(vm.hasData)
    }

    func testClearResetsAndCallsCore() async {
        let mock = MockPomAPI()
        let vm = RenderFlowViewModel(api: mock)
        await vm.clear(branch: "feat")
        XCTAssertEqual(mock.renderClearCalls, 1)
        XCTAssertEqual(vm.summary.targets.count, 0)
    }
}
