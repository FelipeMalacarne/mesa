/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type Connection = {
    id: string;
    name: string;
    driver: Connection.driver;
    host: string;
    port: number;
    username: string;
    createdAt?: string;
    updatedAt?: string;
};
export namespace Connection {
    export enum driver {
        POSTGRESQL = 'postgresql',
        MYSQL = 'mysql',
    }
}

