import { Link, useMatches } from "@tanstack/react-router";
import React from "react";

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";

export function Breadcrumbs() {
  const matches = useMatches();

  // Filter matches that have a breadcrumb in their context
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const breadcrumbs = matches
    .filter((match: any) => match.context?.breadcrumb)
    .filter((match: any, index: number, self: any[]) => {
      // Deduplicate consecutive breadcrumbs with the same label
      if (index === 0) return true;
      return match.context.breadcrumb !== self[index - 1].context.breadcrumb;
    });

  return (
    <Breadcrumb>
      <BreadcrumbList>
        {breadcrumbs.map((match: any, index: number) => {
          const isLast = index === breadcrumbs.length - 1;

          return (
            <React.Fragment key={match.id}>
              <BreadcrumbItem>
                {isLast ? (
                  <BreadcrumbPage>{match.context.breadcrumb}</BreadcrumbPage>
                ) : (
                  <BreadcrumbLink asChild>
                    <Link to={match.pathname} className="hover:text-foreground">
                      {match.context.breadcrumb}
                    </Link>
                  </BreadcrumbLink>
                )}
              </BreadcrumbItem>
              {!isLast && <BreadcrumbSeparator />}
            </React.Fragment>
          );
        })}
      </BreadcrumbList>
    </Breadcrumb>
  );
}
