export const databaseKey = (connectionId: string, databaseName: string) =>
  JSON.stringify([connectionId, databaseName]);

export const parseDatabaseKey = (key: string): [string, string] => {
  const parsed = JSON.parse(key) as [string, string];
  return [parsed[0], parsed[1]];
};
