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
        arView = ARView(frame: CGRect(x: 0, y: 0, width: self.frame.size.width, height: self.frame.size.height))
        self.addSubview(arView)
        // Setup the rest of your AR scene here.
    }
    
    required init?(coder aDecoder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }
}
