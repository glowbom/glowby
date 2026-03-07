import UIKit
import RealityKit
import ARKit

class GlowbyARView: UIView, ARSessionDelegate {
    var arView: ARView!

    override init(frame: CGRect) {
        super.init(frame: frame)
        print("Initializing GlowbyARView with frame: \(frame)")
        
        let arFrame = CGRect(x: frame.width / 2, y: 0, width: frame.width / 2, height: frame.height) // Only cover half the screen
            arView = ARView(frame: arFrame) // Set the frame to the passed value
        arView.session.delegate = self // Set the delegate to receive AR session updates
        arView.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        self.addSubview(arView)

        // Start the AR session with a world tracking configuration
        let configuration = ARWorldTrackingConfiguration()
        configuration.planeDetection = [.horizontal, .vertical] // Add if you want to detect planes
        arView.session.run(configuration)

        // Setup RealityKit scene here
        let anchor = AnchorEntity(world: SIMD3(x: 0.2, y: 0, z: 0)) // Position the cube 20cm to the right of the origin
        let mesh = MeshResource.generateBox(size: 0.1) // Create a 10cm cube
        let material = SimpleMaterial(color: .blue, isMetallic: true)
        let cube = ModelEntity(mesh: mesh, materials: [material])
        anchor.addChild(cube)

        arView.scene.anchors.append(anchor)
    }

    required init?(coder aDecoder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    // ARSessionDelegate method
    func session(_ session: ARSession, didFailWithError error: Error) {
        // Handle session failures
        print("AR Session Failure: \(error.localizedDescription)")
    }

    func sessionWasInterrupted(_ session: ARSession) {
        // Handle session interruptions
        print("AR Session was interrupted")
    }

    func sessionInterruptionEnded(_ session: ARSession) {
        // Handle session interruption ends
        print("AR Session interruption ended")
    }
}
