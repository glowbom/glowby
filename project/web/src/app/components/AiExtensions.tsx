/// AiExtensions.tsx
"use client";
import React from "react";
import Image from "next/image";
import Custom from "@/app/components/Custom";

// These constants are placeholders for AI-generated content.
// To enable the screen, set 'enabled' to true.
// If 'enabled' is false, the screen won't be visible in the app.
// Modify 'title' as per the requirements of your app.
export const enabled = true;
export const title = "Build anything.";

const GlowbyScreen: React.FC = () => {
  // If not enabled, show the default template UI.
  if (!enabled) return <Custom />;

  const characterImage =
    "glowbyimage:Minimal friendly character icon, simple modern mascot face, soft green accent, clean geometric style, light background, app icon aesthetic";

  // Replace the contents of this component with AI-generated content.
  // Ensure that the generated content is centered on the screen
  // and within a maximum width of 360px.
  // always use Image from "next/image" for images
  // the UI should be build using shadcn/ui
  // the following components are available: button, card, dialog, dropdown-menu, input, label, pagination, progress, tabs, toggle, tooltip
  // each component should be imported individually from shadcn/ui only in the following formats: 
  // import { Button } from "@/components/ui/button";
  // import { Card } from "@/components/ui/card"; 
  // import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger, } from "@/components/ui/dialog"
  // import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger, } from "@/components/ui/dropdown-menu"
  // import { Input } from "@/components/ui/input"
  // import { Label } from "@/components/ui/label"
  // import { Pagination, PaginationContent, PaginationEllipsis, PaginationItem, PaginationLink, PaginationNext, PaginationPrevious, } from "@/components/ui/pagination"
  // import { Progress } from "@/components/ui/progress"
  // import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
  // import { Toggle } from "@/components/ui/toggle"
  // import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger, } from "@/components/ui/tooltip"
  return (
    <div className="min-h-screen bg-[#F5F5F5] text-[#111111] selection:bg-[#29DE92]/30">
      <main className="flex min-h-screen items-center justify-center px-6">
        <section className="w-full max-w-[360px] text-center">
          <div className="flex flex-col items-center">
            <div className="mb-8 rounded-full p-3">
              <div className="flex h-24 w-24 items-center justify-center rounded-full bg-white shadow-[0_20px_60px_rgba(0,0,0,0.08)] ring-1 ring-black/5 sm:h-28 sm:w-28">
                <Image
                  src={characterImage}
                  alt="Minimal friendly character icon"
                  width={64}
                  height={64}
                  className="h-14 w-14 rounded-2xl object-cover sm:h-16 sm:w-16"
                  unoptimized
                />
              </div>
            </div>

            <h1 className="text-5xl font-extrabold tracking-tight sm:text-6xl">
              Build{" "}
              <span className="relative inline-block">
                <span>anything.</span>
                <span className="absolute -bottom-2 left-1/2 h-1.5 w-16 -translate-x-1/2 rounded-full bg-[#29DE92]" />
              </span>
            </h1>

            <p className="mt-16 text-sm font-medium tracking-wide text-black/55">
              Made with <span className="text-[#29DE92]">Glowbom</span>
            </p>
          </div>
        </section>
      </main>
    </div>
  );
};

export default GlowbyScreen;