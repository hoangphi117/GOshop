import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold transition-colors focus:outline-none focus:ring-2 focus:ring-[#00ADD8] focus:ring-offset-2",
  {
    variants: {
      variant: {
        default:
          "bg-[#007D9C] text-white shadow-2xs",
        cyan:
          "bg-[#E6F7FB] text-[#007D9C] border border-[#00ADD8]/30",
        accent:
          "bg-[#F26522] text-white shadow-xs",
        secondary:
          "bg-slate-100 text-slate-700 border border-slate-200",
        outline:
          "text-slate-800 border border-slate-200",
        destructive:
          "bg-red-50 text-red-700 border border-red-200",
        success:
          "bg-emerald-50 text-emerald-700 border border-emerald-200",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props} />
  );
}

export { Badge, badgeVariants };
