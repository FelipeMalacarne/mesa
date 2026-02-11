import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { parseDatabaseKey } from "./utils";

export type NavConnectionsContextValue = {
  openConnections: Record<string, boolean>;
  openDatabases: Record<string, boolean>;
  setConnectionOpen: (connectionId: string, open: boolean) => void;
  setDatabaseOpen: (databaseKey: string, open: boolean) => void;
};

const NavConnectionsContext = createContext<NavConnectionsContextValue | null>(
  null,
);

export const NavConnectionsProvider = ({
  children,
}: {
  children: ReactNode;
}) => {
  const [openConnections, setOpenConnections] = useState<Record<string, boolean>>(
    {},
  );
  const [openDatabases, setOpenDatabases] = useState<Record<string, boolean>>({});

  const setConnectionOpen = useCallback((connectionId: string, open: boolean) => {
    setOpenConnections((prev) => ({
      ...prev,
      [connectionId]: open,
    }));

    if (!open) {
      setOpenDatabases((prev) => {
        const next = { ...prev };
        Object.keys(next).forEach((key) => {
          const [id] = parseDatabaseKey(key);
          if (id === connectionId) {
            delete next[key];
          }
        });
        return next;
      });
    }
  }, []);

  const setDatabaseOpen = useCallback((key: string, open: boolean) => {
    setOpenDatabases((prev) => ({
      ...prev,
      [key]: open,
    }));
  }, []);

  const value = useMemo(
    () => ({
      openConnections,
      openDatabases,
      setConnectionOpen,
      setDatabaseOpen,
    }),
    [openConnections, openDatabases, setConnectionOpen, setDatabaseOpen],
  );

  return (
    <NavConnectionsContext.Provider value={value}>
      {children}
    </NavConnectionsContext.Provider>
  );
};

export const useNavConnectionsState = () => {
  const context = useContext(NavConnectionsContext);
  if (!context) {
    throw new Error(
      "useNavConnectionsState must be used within a NavConnectionsProvider",
    );
  }

  return context;
};
