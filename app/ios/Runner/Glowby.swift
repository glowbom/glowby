//
//  Glowby.swift
//  Runner
//
//  Created by Jacob Ilin on 2/2/24.
//

import UIKit
import RealityKit

class GlowbyARView: UIView {
    var arView: ARView!

    override init(frame: CGRect) {
        super.init(frame: frame)
        arView = ARView(frame: .zero) // Initialize with zero frame
        arView.autoresizingMask = [.flexibleWidth, .flexibleHeight] // Use autoresizing instead
        self.addSubview(arView)

        // Setup RealityKit scene here
        let anchor = AnchorEntity(world: .zero) // Corrected to use the right initializer
        let mesh = MeshResource.generateBox(size: 0.1) // Creates a cube of 10cm x 10cm x 10cm
        let material = SimpleMaterial(color: .blue, isMetallic: true)
        let cube = ModelEntity(mesh: mesh, materials: [material])
        anchor.addChild(cube)

        arView.scene.anchors.append(anchor)
    }

    required init?(coder aDecoder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }
}

