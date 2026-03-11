// types.ts
export interface Question {
  id: string;
  title: string;
  description: string;
  buttonsTexts: string[];
  buttonAnswers: number[];
  answersCount: number;
  goIndexes: number[];
  answerPicture: string;
  answerPictureDelay: number;
  goConditions: any[];
  heroValues: any[];
  picturesSpriteNames: string[];
}

export interface CustomContent {
  questions: Question[];
}
