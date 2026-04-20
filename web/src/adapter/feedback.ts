import message from 'ant-design-vue/es/message'

export function notifySuccess(content: string) {
  void message.success(content)
}

export function notifyError(content: string) {
  void message.error(content)
}

export function notifyInfo(content: string) {
  void message.info(content)
}
