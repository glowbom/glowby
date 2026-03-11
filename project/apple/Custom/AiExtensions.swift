import SwiftUI

struct GlowbyScreen: View {
    static let enabled = true
    static let title = "App"

    @State private var animateIcon = false

    var body: some View {
        ZStack {
            Color(red: 245 / 255, green: 245 / 255, blue: 245 / 255)
                .ignoresSafeArea()

            VStack {
                Spacer()
                HStack {
                    Spacer()

                    VStack(spacing: 0) {
                        Button {
                            withAnimation(.easeInOut(duration: 0.18)) {
                                animateIcon = true
                            }
                            DispatchQueue.main.asyncAfter(deadline: .now() + 0.28) {
                                withAnimation(.easeInOut(duration: 0.18)) {
                                    animateIcon = false
                                }
                            }
                        } label: {
                            ZStack {
                                Circle()
                                    .fill(Color(red: 41 / 255, green: 222 / 255, blue: 146 / 255).opacity(animateIcon ? 0.12 : 0.18))
                                    .frame(width: animateIcon ? 108 : 96, height: animateIcon ? 108 : 96)

                                Circle()
                                    .fill(Color.white)
                                    .frame(width: 96, height: 96)
                                    .shadow(color: .black.opacity(0.08), radius: 30, x: 0, y: 12)
                                    .overlay(
                                        Circle()
                                            .stroke(Color.black.opacity(0.05), lineWidth: 1)
                                    )

                                Image(systemName: "face.smiling")
                                    .font(.system(size: 36, weight: .regular))
                                    .foregroundStyle(Color(red: 41 / 255, green: 222 / 255, blue: 146 / 255))
                                    .scaleEffect(animateIcon ? 1.1 : 1.0)
                                    .rotationEffect(.degrees(animateIcon ? 3 : 0))
                            }
                        }
                        .buttonStyle(.plain)
                        .padding(.bottom, 32)

                        VStack(spacing: 16) {
                            VStack(spacing: 6) {
                                Text("Build")
                                    .font(.system(size: 44, weight: .heavy))
                                    .foregroundStyle(Color(red: 17 / 255, green: 17 / 255, blue: 17 / 255))

                                VStack(spacing: 8) {
                                    Text("anything.")
                                        .font(.system(size: 44, weight: .heavy))
                                        .foregroundStyle(Color(red: 17 / 255, green: 17 / 255, blue: 17 / 255))

                                    Capsule()
                                        .fill(Color(red: 41 / 255, green: 222 / 255, blue: 146 / 255))
                                        .frame(width: 72, height: 6)
                                }
                            }
                            .multilineTextAlignment(.center)

                            Text("Made with ")
                                .font(.system(size: 14, weight: .medium))
                            + Text("Glowbom")
                                .font(.system(size: 14, weight: .medium))
                        }
                    }
                    .frame(maxWidth: 360)

                    Spacer()
                }
                Spacer()
            }
        }
    }
}