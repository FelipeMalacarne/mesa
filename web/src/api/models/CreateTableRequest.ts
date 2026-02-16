/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { CreateTableColumn } from './CreateTableColumn';
import type { CreateTableIndex } from './CreateTableIndex';
export type CreateTableRequest = {
    name: string;
    columns: Array<CreateTableColumn>;
    indexes?: Array<CreateTableIndex>;
};

