"use client";

import React from "react";

export function MainContainer({ children }: { children: React.ReactNode }) {
  return (
    <main className="flex-1 flex flex-col min-w-0 w-full">
      {children}
    </main>
  );
}
