import { cn } from "@/lib/utils";

interface ContainerProps {
  children: React.ReactNode;
  // narrow caps the column for reading/forms (apply, status); default uses the
  // full responsive site container.
  narrow?: boolean;
  className?: string;
}

// Container centers content within the responsive site width with fluid padding.
// Default: max-w-[var(--container)] (1200px). narrow: a comfortable form column.
export function Container({ children, narrow, className }: ContainerProps) {
  return (
    <div
      className={cn(
        "mx-auto w-full px-4 sm:px-6 lg:px-8",
        narrow ? "max-w-xl" : "max-w-[var(--container)]",
        className,
      )}
    >
      {children}
    </div>
  );
}
