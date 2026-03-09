import { Github } from "lucide-react";
import { DISCORD_URL, GITHUB_URL } from "../lib/site";

const discordIconSrc = `${import.meta.env.BASE_URL}discord-icon.png`;

const actionClassName =
  "inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-fd-muted-foreground transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground";

export function NavActions() {
  return (
    <div className="me-1 flex h-8 items-center gap-1">
      <a
        aria-label="GitHub"
        className={actionClassName}
        href={GITHUB_URL}
        rel="noreferrer"
        target="_blank"
      >
        <Github className="block size-[18px]" />
      </a>
      <a
        aria-label="Discord"
        className={actionClassName}
        href={DISCORD_URL}
        rel="noreferrer"
        target="_blank"
      >
        <img
          alt=""
          aria-hidden="true"
          className="block size-[18px] rounded-[4px]"
          height={18}
          src={discordIconSrc}
          width={18}
        />
      </a>
    </div>
  );
}
