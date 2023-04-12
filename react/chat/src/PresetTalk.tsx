import React, { useState, useEffect } from "react";
import ChatScreen from "./ChatScreen";

interface TalkProps {
  content: any;
}

const Talk: React.FC<TalkProps> = ({ content }) => {
  const [appScreen, setAppScreen] = useState("Loading");
  const [title, setTitle] = useState<string | null>(null);
  const [mainColor, setMainColor] = useState("Blue");
  const [voice, setVoice] = useState(false);
  const [questions, setQuestions] = useState<Array<{ [key: string]: any }>>([]);

  useEffect(() => {
    async function loadContent() {
      try {
        const response = await fetch("/talk.glowbom");
        const data = await response.json();

        setTitle(data.title);
        setMainColor(data.main_color || "Blue");
        setVoice(data.voice || false);
        setQuestions(data.questions);
        setAppScreen("Test100");
      } catch (error) {
        console.error("Error loading content:", error);
      }
    }

    if (content) {
      setTitle(content.title);
      setMainColor(content.main_color || "Blue");
      setVoice(content.voice || false);
      setQuestions(content.questions);
      setAppScreen("Test100");
    } else {
      loadContent();
    }
  }, [content]);

  if (appScreen === "Loading") {
    return <div>Loading...</div>;
  } else if (appScreen === "Glowbom") {
    return <div><img src="/glowbom.png" alt="Glowbom" /></div>;
  } else {
    return (
      <ChatScreen name={content?.start_over || "AI"} questions={questions} voice={voice} mainColor={mainColor} title={title}/>
    );
  }
};

export default Talk;
