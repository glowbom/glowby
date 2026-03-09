//
//  ContentView.swift
//  Custom
//
//  Created by Jacob Ilin on 7/29/23.
//

import SwiftUI

struct Question: Identifiable {
    let id = UUID()
    let title: String
    let description: String
    let buttonsTexts: [String]
    let buttonAnswers: [Int]
    let answersCount: Int
    let goIndexes: [Int]
    let answerPicture: String
    let answerPictureDelay: Int
    let goConditions: [Any]
    let heroValues: [Any]
    let picturesSpriteNames: [String]

    init(dictionary: [String: Any]) {
        self.title = dictionary["title"] as? String ?? ""
        self.description = dictionary["description"] as? String ?? ""
        if let buttonsTextsAny = dictionary["buttonsTexts"] as? [Any] {
            self.buttonsTexts = buttonsTextsAny.map { String(describing: $0) }
        } else {
            self.buttonsTexts = []
        }
        self.buttonAnswers = dictionary["button_answers"] as? [Int] ?? []
        self.answersCount = dictionary["answers_count"] as? Int ?? 0
        self.goIndexes = dictionary["go_indexes"] as? [Int] ?? []
        self.answerPicture = dictionary["answer_picture"] as? String ?? ""
        self.answerPictureDelay = dictionary["answer_picture_delay"] as? Int ?? 0
        self.goConditions = dictionary["go_conditions"] as? [Any] ?? []
        self.heroValues = dictionary["hero_values"] as? [Any] ?? []
        self.picturesSpriteNames = dictionary["pictures_sprite_names"] as? [String] ?? []
    }
}

struct Custom: View {
    @State private var appScreen: String = "Loading"
    @State private var content: [String: Any]?
    @State private var title: String = "App"
    @State private var mainColor: Color = .blue
    @State private var questions: [Question] = []
    @State private var isLoading: Bool = true
    
    init(content: [String: Any]?) {
        self._content = State(initialValue: content)
        self._title = State(initialValue: "App")
        self._mainColor = State(initialValue: .blue)
    }
    
    func loadContentFromAssets() {
        guard let url = Bundle.main.url(forResource: "custom", withExtension: "glowbom") else {
            print("Could not find custom.glowbom")
            return
        }
        
        guard let data = try? Data(contentsOf: url) else {
            print("Could not load data from custom.glowbom")
            return
        }
        
        do {
            let json = try JSONSerialization.jsonObject(with: data, options: [])
            if let jsonDict = json as? [String: Any] {
                DispatchQueue.main.async {
                    self.content = jsonDict
                    self.initializeCustomState()
                }
            } else {
                print("Invalid JSON format")
            }
        } catch {
            print("JSON decoding error: \(error)")
        }
    }
    
    func initializeCustomState() {
        if let content = content {
            title = content["title"] as? String ?? "App"
            mainColor = Color(content["main_color"] as? String ?? "blue")
            questions = (content["questions"] as? [[String: Any]])?.map { Question(dictionary: $0) } ?? []
            self.isLoading = false
            self.appScreen = "Questions"
        } else {
            loadContentFromAssets()
        }
    }

    var body: some View {
        GeometryReader { geometry in
            let topPadding = GlowbyScreen.enabled ? 0 : geometry.size.height * 0.1
            VStack {
                if isLoading {
                    Text("Loading...")
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)
                } else if appScreen == "Glowbom" {
                    Image("glowbom")
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)
                } else if GlowbyScreen.enabled {
                    GlowbyScreen()
                } else if appScreen == "Questions" {
                    ScrollView {
                        VStack {
                            ForEach(questions) { question in
                                switch question.description {
                                case "Image":
                                    let urls = question.buttonsTexts.compactMap { URL(string: $0) }

                                    if let imageUrl = urls.first {
                                        AsyncImage(url: imageUrl) { phase in
                                            switch phase {
                                            case .success(let image):
                                                image
                                                    .resizable()
                                                    .aspectRatio(contentMode: .fit)
                                                    .frame(maxWidth: 320, maxHeight: 200)
                                                    .padding()
                                            case .failure:
                                                Text("Failed to load image from URL: \(imageUrl)")
                                            case .empty:
                                                ProgressView()
                                            @unknown default:
                                                ProgressView()
                                            }
                                        }
                                    } else {
                                        Text("No valid URL found in buttonsTexts")
                                    }

                                case "Text":
                                    Text(question.buttonsTexts.first ?? "")
                                        .padding()
                                        .frame(maxWidth: 320)

                                case "Button":
                                    if let buttonTitle = question.buttonsTexts.first, let urlString = question.buttonsTexts.dropFirst().first, let url = URL(string: urlString) {
                                        Link(destination: url) {
                                            Text(buttonTitle)
                                        }
                                        .padding()
                                        .frame(maxWidth: 320)
                                    } else {
                                        Button(action: {}) {
                                            Text(question.buttonsTexts.first ?? "")
                                        }
                                        .padding()
                                        .frame(maxWidth: 320)
                                    }
                                default:
                                    Text("Unsupported question type")
                                }
                            }
                        }
                        .frame(maxWidth: .infinity)
                        .frame(minHeight: geometry.size.height)
                        .frame(maxWidth: .infinity, alignment: .center)
                    }
                } else {
                    Text("Loaded, but appScreen is '\(appScreen)'")
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .padding(.top, topPadding)
            .onAppear {
                if content == nil {
                    loadContentFromAssets()
                }
            }
        }
    }
}

struct ContentView: View {
    var body: some View {
        Custom(content: nil)
    }
}
