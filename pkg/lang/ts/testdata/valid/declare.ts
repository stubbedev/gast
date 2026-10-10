export {};

declare const version: string;
declare let counter: number;
declare function configure(options: object): void;
declare class Socket {
  constructor(url: string);
  send(data: string | ArrayBuffer): void;
  close(): void;
}
declare const enum Mode {
  Read,
  Write,
}
declare namespace DOM {
  export const ready: boolean;
  export function query(selector: string): HTMLElement | null;
}
declare namespace global {
  export const appId: string;
}
declare abstract class BaseModel<T> {
  abstract id: T;
  abstract save(): Promise<void>;
  describe(): string;
}
declare interface Repository<T> {
  find(id: string): Promise<T>;
}
declare type Alias = number | string;

configure({ ready: true });
const ready: boolean = DOM.ready;
const repo: Repository<string> | null = null;
console.log(version, counter, Socket, DOM.query("x"), global.appId, ready, repo, null as unknown as Alias);
