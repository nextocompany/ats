import { cn } from "@/lib/utils";

interface ContainerProps {
  children: React.ReactNode;
  // narrow caps the column for reading/forms (apply, status, auth); default uses
  // the full institutional container (1200px).
  narrow?: boolean;
  className?: string;
  as?: "div" | "section" | "header" | "footer";
}

// Container centers content within the institutional grid width with generous,
// fluid gutters. Default: max-w-[var(--container)] (1200px). narrow: a focused
// reading/form column.
export function Container({ children, narrow, className, as: Tag = "div" }: ContainerProps) {
  return (
    <Tag
      className={cn(
        "mx-auto w-full px-5 sm:px-8 lg:px-10",
        narrow ? "max-w-[40rem]" : "max-w-[var(--container)]",
        className,
      )}
    >
      {children}
    </Tag>
  );
}
