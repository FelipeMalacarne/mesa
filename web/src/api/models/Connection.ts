/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type Connection = {
    id: string;
    name: string;
    driver: Connection.driver;
    host: string;
    /**
     * A port value between 0 and 65535
     */
    port: number;
    username: string;
    createdAt?: string;
    updatedAt?: string;
    status?: Connection.status;
    status_error?: string;
};
export namespace Connection {
    export enum driver {
        POSTGRES = 'postgres',
        MYSQL = 'mysql',
    }
    export enum status {
        OK = 'ok',
        ERROR = 'error',
    }
}

