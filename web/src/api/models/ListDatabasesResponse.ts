/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { Database } from './Database';
export type ListDatabasesResponse = {
    status: ListDatabasesResponse.status;
    error?: string;
    databases: Array<Database>;
};
export namespace ListDatabasesResponse {
    export enum status {
        OK = 'ok',
        ERROR = 'error',
    }
}

