// TextToSpeech.ts
class TextToSpeech {
    private languageCodes: { [key: string]: string } = {
      Italian: "it-IT",
      German: "de-DE",
      Portuguese: "pt-PT",
      Dutch: "nl-NL",
      Russian: "ru-RU",
      "American Spanish": "es-US",
      "Mexican Spanish": "es-MX",
      "Canadian French": "fr-CA",
      French: "fr-FR",
      Spanish: "es-ES",
      "American English": "en-US",
      "British English": "en-GB",
      "Australian English": "en-AU",
      English: "en-US",
    };
  
    speakText(text: string): Promise<void> {
      return new Promise((resolve, reject) => {
        const utterance = new SpeechSynthesisUtterance();
  
        for (const [key, value] of Object.entries(this.languageCodes)) {
          if (text.startsWith(`${key}: `)) {
            utterance.lang = value;
            text = text.replace(`${key}: `, "");
            break;
          }
        }
  
        utterance.text = text;
        utterance.onend = () => resolve();
        utterance.onerror = (event) => reject(new Error(`Error speaking the text: ${event}`));
  
        speechSynthesis.speak(utterance);
      });
    }
  }
  
  export default TextToSpeech;
  