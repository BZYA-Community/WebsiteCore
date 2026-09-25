declare namespace Api {

    namespace Attachment {

        interface Api {
            get: {
                /** 获取附件 */
                _self: (params: NetParams.UserGetAttachment) => Promise<NetReq.UserGetAttachment>;
            }
        }

        namespace NetParams {

            interface UserGetAttachment {
                id: number;
            }
        }

        namespace NetReq {

            interface UserGetAttachment {
                signed_url: string;
            }
        }

    }

}