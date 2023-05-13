# Glowby Basic - Create Voice AI Assistant App in Minutes

Glowby Basic is a powerful voice-based AI assistant that can help users with various tasks. Easily customizable, trainable, and deployable anywhere, Glowby Basic is designed to adapt to your specific needs. Built using Flutter, Glowby Basic provides a seamless web app experience with an intuitive voice interface.

[![Deploy to Vercel](https://vercel.com/button)](https://vercel.com/import/git?s=https://github.com/glowbom/glowby-basic)

## Live Demo 🤖

Experience Glowby Basic in action with our live demo hosted on GitHub Pages [here](https://glowbom.github.io/glowby-basic/).


![GitHub Repo stars](https://img.shields.io/github/stars/glowbom/glowby?style=social)
[![Twitter Follow](https://img.shields.io/twitter/follow/GlowbomCorp?style=social)](https://twitter.com/GlowbomCorp)
[![Discord Follow](https://dcbadge.vercel.app/api/server/jpWW6vB4Jk?style=flat)](https://discord.gg/jpWW6vB4Jk)
[![MIT License](https://img.shields.io/badge/license-MIT-blue)](https://github.com/glowbom/glowby/blob/main/LICENSE)


## See It in Action 

### Experimental Autonomous Mode 🧠

![Glowby Basic Experimental Autonomous Mode](https://user-images.githubusercontent.com/2455891/233245896-59d5f7a4-667c-4f74-95c0-b348a3712e9e.gif)
Glowby plans a trip to Portugal with Autonomous Mode. To see this demo with sound, check out this [Twitter post](https://twitter.com/jacobilin/status/1648870682972004352).

### Regular Mode 🤖

![Glowby Basic Demo](https://user-images.githubusercontent.com/2455891/232182586-30984d36-4641-41da-9e1e-c23c27716e3d.gif)

## Overview

This project offers an easy way for creating customizable AI assistants like [Glowby](https://www.youtube.com/watch?v=iFECpMXmKOg), a witty AI agent that assists users in building apps on [Glowbom.com](https://www.glowbom.com). By open-sourcing the Flutter-based chat component, we aim to foster a community-driven ecosystem to build diverse AI agents for a variety of use cases.

## Features

- **New!** Experimental Autonomous Mode (watch a [quick demo](https://twitter.com/jacobilin/status/1648870682972004352))
- **New!** Image Generation powered by **DALL·E** (watch a [quick demo](https://twitter.com/jacobilin/status/1649910120988696576))
- **New!** Multilingual Translations (watch a [quick demo](https://twitter.com/jacobilin/status/1651084311431622658))
- **New!** Interactive Adventure (watch a [quick demo](https://twitter.com/jacobilin/status/1651449595447369729))
- Powerful, customizable voice-based AI assistant
- Pre-set questions and answers using the [Glowbom builder](https://www.glowbom.com)
- Voice input and output for a smooth and intuitive user experience
- Customizable prompts allowing you to tailor the assistant to your needs
- Easily switch between different prompts for a variety of scenarios and tasks
- Support for multiple languages: American English, American Spanish, Argentinian Spanish, Arabic (Saudi Arabia), Australian English, Brazilian Portuguese, British English, Bulgarian, Canadian French, Chinese (Simplified), Chinese (Traditional), Czech, Danish, Dutch, English, Finnish, French, German, Greek, Hebrew (Israel), Hungarian, Indonesian, Italian, Japanese, Korean, Mexican Spanish, Norwegian, Polish, Portuguese, Romanian, Russian, Slovak, Spanish, Swedish, Thai, Turkish, Ukrainian, and Vietnamese. Want to add more languages? Feel free to let us know on [Twitter](https://twitter.com/glowbomcorp).

More details about Glowby Basic's features in our Twitter thread [here](https://twitter.com/jacobilin/status/1649443429347397632). 

### Multilingual Support in Action

![Glowby Basic Demo](https://user-images.githubusercontent.com/2455891/232395321-cea05b32-070d-494a-ac85-05c5f493f2ba.gif)

To experience the Autonomous Mode demo with sound, check out this [Twitter post](https://twitter.com/jacobilin/status/1648870682972004352).

### Switch Between Different Prompts

![Glowby Basic Demo](https://user-images.githubusercontent.com/2455891/232727678-ced2ee44-a5df-45da-8846-d90e82c8a007.gif)


### Image Generation powered by DALL·E

![Glowby Basic Image Generation](https://user-images.githubusercontent.com/2455891/233778560-47d8e011-03a8-41ad-a1fc-33827a033475.gif)


## Upcoming Features

We're constantly working to improve our project and have several exciting features in development. Here's a sneak peek at what's coming soon:

### Functionality
- Local Storage
- Backend Service for API Calls

### Monetization
- Adding a paywall

Stay tuned for more updates and enhancements as we continue to grow and develop the project!


## Getting Started

### Prerequisites

- Flutter SDK (version 3.7.10 or higher)
- Dart (version 2.19.5 or higher)
- A compatible browser or device for running the web app
- [OpenAI API key](https://platform.openai.com/account/api-keys)

Glowby Basic supports **GPT-4** and **GPT-3.5** models. If you don't have access to **GPT-4**, you can join the waitlist [here](https://help.openai.com/en/articles/7102672-how-can-i-access-gpt-4).

Glowby Basic is powered by the **ChatGPT API** and built on the same groundbreaking technology as **ChatGPT**, the industry-leading natural-language AI system from OpenAI. It also utilizes OpenAI's **GPT-4**, the most advanced AI system that offers broad general knowledge and domain expertise, can follow complex instructions in natural language, and solve difficult problems with accuracy.

Glowby Basic supports image generation powered by **DALL·E**. OpenAI’s **DALL·E 2** system generates unique images and art from text descriptions, combining concepts and styles in creative and inspiring ways.

### Installation

1. Clone the repository:

```
git clone https://github.com/glowbom/glowby.git 
```

2. Navigate to the project directory:

```
cd app
```

3. Install dependencies:


```
flutter pub get
```

4. Run the project in your preferred environment:


```
flutter run
```

## Deployment

Glowby Basic comes with a pre-built `dist` folder, which you can deploy directly to your preferred hosting platform. Alternatively, you can build the project yourself and deploy the output. Glowby Basic is compatible with a variety of hosting services, including Netlify, Vercel, Firebase, AWS, and more. Simply follow the deployment instructions provided by your chosen hosting service. Compiled code is available in a separate GitHub project [here](https://github.com/glowbom/glowby-basic).

[![Deploy to Vercel](https://vercel.com/button)](https://vercel.com/import/git?s=https://github.com/glowbom/glowby-basic)

## Customization

To customize the AI assistant's behavior and tasks, modify the default prompt in AI Settings or in the code.

#### Complex Task Prompt
```
You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. For each step, you offer the user 3 options to choose from. Once the user selects an option, you proceed to the next step based on their choice. After the user has chosen an option for the fifth step, you provide them with a customized, actionable plan based on their previous responses. You only reveal the current step and options to ensure an engaging, interactive experience.
```

#### Habit Formation
```
Act as a dual PhD in sports psychology and neuroscience. Your job is to design a system that gets someone addicted to a positive habit, starting with the user's input. Create a concise, actionable plan using research-backed principles to help anyone build a habit if they follow the plan. Incorporate research such as BF Skinner\'s study of addiction, BJ Fogg's Behavioral Model, and similar research on addiction and compulsion. Be concise yet informative. Give a concise day-by-day plan for the first week. Your response should be fewer than 10 sentences.
```

#### Brainstorming Prompt
```
Generate ideas with Glowby! As a super helpful, nice, and humorous AI assistant, Glowby is ready to provide you with a concise plan and assist in executing it. With Glowby by your side, you'll never feel stuck again. Let's get brainstorming!
```

#### Simple Assistant Prompt
```
You are Glowby, super helpful, nice, and humorous AI assistant ready to help with anything. I like to joke around.
```

#### Stand-up Comedy Prompt
```
You are Glowby, a hilarious AI stand-up comedian, skilled in creating funny conversations that become popular on social media platforms like Reels. Users can provide you with a topic, and you'll craft witty one-liners, puns, or dialogues that make people laugh out loud. Your jokes should be light-hearted, engaging, and suitable for cartoon adaptation. Let's get the laughs rolling!
```

#### Creative Writing Prompt
```
You are Glowby, a talented AI writer who helps users craft engaging and imaginative stories. Provide a captivating opening scene or a plot twist that will inspire users to develop their own unique stories.
```

#### Problem Solving Prompt
```
You are Glowby, a resourceful AI assistant skilled in finding solutions to various problems. Users can present you with a challenge, and you'll help them brainstorm practical, step-by-step solutions to overcome it.'
```

#### Learning and Education Prompt
```
You are Glowby, an AI tutor who assists users with their learning needs. Users can ask questions about a wide range of subjects, and you'll provide clear, concise explanations to help them understand the topic better.
```

#### Career and Job Advice Prompt
```
You are Glowby, an AI career coach who offers guidance on job-related matters. From resume tips to interview techniques, you provide personalized advice to users seeking professional growth and success.
```

#### Daily Motivation Prompt
```
You are Glowby, an AI life coach who delivers daily doses of inspiration and motivation. Users can rely on you for uplifting quotes, insightful advice, and practical tips to help them stay positive and focused on their goals.
```

Want to add your prompt? Let us know on [Twitter](https://twitter.com/glowbomcorp).


### Questions Pre-set

One of the powerful features of Glowby Basic is the ability to pre-set questions and answers for your AI assistant. Using [Glowbom.com](https://www.glowbom.com), you can create a knowledge base of questions and answers that your AI assistant can use to provide instant responses. 
![Glowby Basic Demo](https://user-images.githubusercontent.com/2455891/232735288-abb5f9d8-3d51-4170-a6dd-a967e7d8ae30.gif)

If the answer to a question is not found locally, the app will make a server request to retrieve the relevant information, ensuring that users receive accurate and helpful responses.

### AI-Agent Settings

Using [Glowbom.com](https://www.glowbom.com), you can customize your AI-agent settings to meet your specific needs. In the **Glowbom** AI-agent project editor, select your preferred AI model, tweak system prompts, and choose the voice for your voice-based AI-agent app.

![AI Agent Settings](https://github.com/glowbom/glowby/assets/2455891/e82bf935-d7d3-4073-b072-a89c1c96b130)

After setting up your preferences on [Glowbom.com](https://www.glowbom.com), you can easily import the settings into Glowby Basic. Simply import the Glowbom data file using the **Import** button located at the top right corner of the Glowbom project editor. To make sure your changes take effect in your Glowby Basic project, remember to replace the existing data file in the **assets** folder with your new **Glowbom** data file.

![Glowby Basic Import](https://github.com/glowbom/glowby/assets/2455891/462c9915-1d63-4a10-9fa5-31965d4df33a)


### Autonomous Mode (Experimental)

![Glowby Basic Demo](https://user-images.githubusercontent.com/2455891/233034444-9457c62c-3fc3-47f3-bd08-198093ea9c76.gif)

To experience the Autonomous Mode demo with sound, check out this [Twitter post](https://twitter.com/jacobilin/status/1648870682972004352).

### Interactive Adventure

![Interactive Adventure](https://user-images.githubusercontent.com/2455891/234763801-0b636f31-74e1-4f57-8978-a52938981f14.gif)

It lets you embark on captivating, choice-driven narratives, where your decisions shape the story.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=glowbom/glowby&type=Date)](https://star-history.com/#glowbom/glowby&Date)

### ⭐️ Star Us ⭐️
If this project helps you create voice AI assistant apps faster, please consider starring it – every star makes us very happy!

## Contributing

We're excited to have you join our community and contribute to Glowby Basic! Whether you're interested in fixing bugs, adding new features, or improving documentation, your contributions are welcome. Feel free to open issues and submit pull requests on GitHub.


## License

Glowby Basic is released under the [MIT License](https://opensource.org/licenses/MIT).

## Contact

If you have any questions or need assistance, feel free to reach out to us on [Twitter](https://twitter.com/glowbomcorp).

