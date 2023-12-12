const SpeechRecognition =
  window.SpeechRecognition || window.webkitSpeechRecognition;

if (SpeechRecognition != null) {
  const r = new SpeechRecognition();
  r.lang = "en-US";
  r.interimResults = false;

  r.addEventListener("result", (e) => {
    let last = e.results.length - 1;
    let text = e.results[last][0].transcript;

    //console.log("last: " + last);
    //console.log("text: " + text);

    //console.log("Confidence: " + e.results[0][0].confidence);

    vr(text);
  });

  function rv(lang) {
    if (typeof lang !== "undefined" && lang !== null) {
      r.lang = lang;
      //console.log("switched to " + lang);
    }

    r.start();
    //console.log("recordVoice call");
  }
} else {
  //console.error("SpeechRecognition API not supported in this browser.");
}
