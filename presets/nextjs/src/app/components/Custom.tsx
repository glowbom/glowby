"use client";
import React, { useState, useEffect } from "react";
import { Question, CustomContent } from "@/models/types";
import { Button } from "@/components/ui/button";
import Image from "next/image";

const Custom: React.FC = () => {
  const [appScreen, setAppScreen] = useState("Loading");
  const [content, setContent] = useState<CustomContent | null>(null);

  useEffect(() => {
    const loadContentFromAssets = async () => {
      try {
        const response = await fetch("/custom.glowbom");
        const data: CustomContent = await response.json();
        setContent(data);
        setAppScreen("Questions"); // Set the screen state based on your conditions
      } catch (error) {
        console.error("Failed to load content:", error);
        setAppScreen("Error");
      }
    };

    loadContentFromAssets();
  }, []);

  const renderQuestion = (question: Question) => {
    return (
      <div className="image-card">
        {question.description === "Image" && (
          <Image
            src={question.buttonsTexts[0]}
            alt={question.title}
            width={1200}
            height={800}
            unoptimized
            sizes="(max-width: 768px) 100vw, 42rem"
            className="h-auto w-full rounded-xl object-cover"
          />
        )}

        {question.description === "Text" && (
          <p className="text-width">{question.buttonsTexts[0]}</p>
        )}

        {question.description === "Button" && (
          <div className="flex justify-center">
            <Button
              onClick={() => {
                if (question.buttonsTexts.length > 1) {
                  window.open(
                    question.buttonsTexts[1],
                    "_blank",
                    "noopener,noreferrer"
                  );
                }
              }}
              className="btn-width text-center"
            >
              {question.buttonsTexts[0]}
            </Button>
          </div>
        )}
      </div>
    );
  };

  if (appScreen === "Loading") {
    return <div>Loading...</div>;
  }

  if (appScreen === "Error" || !content) {
    return <div>Error loading content.</div>;
  }

  return (
    <div className="container mt-20">
      {appScreen === "Loading" && <div>Loading...</div>}
      {appScreen === "Error" && <div>Error loading content.</div>}
      {appScreen === "Questions" && content?.questions.map(renderQuestion)}
    </div>
  );
};

export default Custom;
