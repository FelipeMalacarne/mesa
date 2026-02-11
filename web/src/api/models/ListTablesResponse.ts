/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { Table } from './Table';
export type ListTablesResponse = {
    status: ListTablesResponse.status;
    error?: string;
    tables: Array<Table>;
};
export namespace ListTablesResponse {
    export enum status {
        OK = 'ok',
        ERROR = 'error',
    }
}

