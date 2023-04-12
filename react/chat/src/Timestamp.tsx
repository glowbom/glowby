class Timestamp {
    private readonly _seconds: number;
    private readonly _nanoseconds: number;
  
    constructor(seconds: number, nanoseconds: number) {
      this._validateRange(seconds, nanoseconds);
      this._seconds = seconds;
      this._nanoseconds = nanoseconds;
    }
  
    static fromMillisecondsSinceEpoch(milliseconds: number): Timestamp {
      const seconds = Math.floor(milliseconds / 1000);
      const nanoseconds = (milliseconds - seconds * 1000) * 1000000;
      return new Timestamp(seconds, nanoseconds);
    }
  
    static fromMicrosecondsSinceEpoch(microseconds: number): Timestamp {
      const seconds = Math.floor(microseconds / 1000000);
      const nanoseconds = (microseconds - seconds * 1000000) * 1000;
      return new Timestamp(seconds, nanoseconds);
    }
  
    static fromDate(date: Date): Timestamp {
      return Timestamp.fromMicrosecondsSinceEpoch(date.getTime() * 1000);
    }
  
    static now(): Timestamp {
      return Timestamp.fromMicrosecondsSinceEpoch(Date.now() * 1000);
    }
  
    get seconds(): number {
      return this._seconds;
    }
  
    get nanoseconds(): number {
      return this._nanoseconds;
    }
  
    get millisecondsSinceEpoch(): number {
      return Math.floor(this._seconds * 1000 + this._nanoseconds / 1000000);
    }
  
    get microsecondsSinceEpoch(): number {
      return Math.floor(this._seconds * 1000000 + this._nanoseconds / 1000);
    }
  
    toDate(): Date {
      return new Date(this.microsecondsSinceEpoch / 1000);
    }
  
    private _validateRange(seconds: number, nanoseconds: number): void {
      if (nanoseconds < 0 || nanoseconds >= 1000000000) {
        throw new Error(`Timestamp nanoseconds out of range: ${nanoseconds}`);
      }
      if (seconds < -62135596800 || seconds >= 253402300800) {
        throw new Error(`Timestamp seconds out of range: ${seconds}`);
      }
    }
  }
  
  export { Timestamp };
  