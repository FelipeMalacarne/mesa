/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type CreateTableIndex = {
    name: string;
    columns: Array<string>;
    unique?: boolean;
    method?: CreateTableIndex.method;
};
export namespace CreateTableIndex {
    export enum method {
        BTREE = 'btree',
        HASH = 'hash',
        GIN = 'gin',
        GIST = 'gist',
        BRIN = 'brin',
        SPGIST = 'spgist',
    }
}

