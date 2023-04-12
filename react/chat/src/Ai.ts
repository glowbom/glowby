import { Timestamp } from './Timestamp';
import { Message } from './Message';

class Ai {
    private _questions?: Array<{ [key: string]: any }>;
    private _name?: string;
  
    static defaultUserId = '007';
  
    constructor(name?: string, questions?: Array<{ [key: string]: any }>) {
      this._name = name;
      this._questions = questions;
    }

  async message(message: string): Promise<Message[]> {
    const foundQuestions = this._findMatchingQuestions(message);

    if (foundQuestions.length > 0) {
      return this._generateResponseMessage(foundQuestions);
    }

    return [];
  }

  private _findMatchingQuestions(message: string): Array<{ [key: string]: any }> {
    const foundQuestions: Array<{ [key: string]: any }> = [];
    const userMessage = this._sanitizeMessage(message);

    if (this._questions) {
      for (const questionMap of this._questions) {
        const question = this._sanitizeMessage(questionMap['description'].toString());

        if (question === userMessage) {
          foundQuestions.push(questionMap);
          break;
        }
      }
    }

    if (foundQuestions.length === 0) {
      foundQuestions.push(...this._searchForQuestions(userMessage));
    }

    return foundQuestions;
  }

  private _sanitizeMessage(message: string): string {
    return message.replace('?', '').toLowerCase();
  }

  private _searchForQuestions(userMessage: string): Array<{ [key: string]: any }> {
    const foundQuestions: Array<{ [key: string]: any }> = [];

    if (this._questions) {
      for (const questionMap of this._questions) {
        const question = this._sanitizeMessage(questionMap['description'].toString());

        if (userMessage.includes(question)) {
          foundQuestions.push(questionMap);
        }
      }
    }

    return foundQuestions;
  }

  private async _generateResponseMessage(
    foundQuestions: Array<{ [key: string]: any }>,
  ): Promise<Message[]> {
    try {
      const rnd = Math.random();
      const messages: string[] = [];

      for (const questionMap of foundQuestions) {
        messages.push(...(questionMap['buttonsTexts'] as Array<string>));
      }

      const index = Math.floor(rnd * messages.length);

      return [
        new Message({
          text: messages[index],
          createdAt: Timestamp.now(),
          userId: Ai.defaultUserId,
          username: this._name === '' ? 'AI' : this._name,
        }),
      ];
    } catch (e) {
      console.error('Error generating response message:', e);
    }

    return [];
  }
}

export default Ai;
