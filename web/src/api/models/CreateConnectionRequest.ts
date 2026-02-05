/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type CreateConnectionRequest = {
    name: string;
    driver: CreateConnectionRequest.driver;
    host: string;
    port: number;
    username: string;
    password: string;
};
export namespace CreateConnectionRequest {
    export enum driver {
        POSTGRESQL = 'postgresql',
        MYSQL = 'mysql',
    }
}

