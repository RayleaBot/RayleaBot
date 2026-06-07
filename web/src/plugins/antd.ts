import type { App } from 'vue'

import AntdApp from 'ant-design-vue/es/app'
import Badge from 'ant-design-vue/es/badge'
import Button from 'ant-design-vue/es/button'
import Card from 'ant-design-vue/es/card'
import Checkbox from 'ant-design-vue/es/checkbox'
import ConfigProvider from 'ant-design-vue/es/config-provider'
import Descriptions from 'ant-design-vue/es/descriptions'
import Divider from 'ant-design-vue/es/divider'
import Drawer from 'ant-design-vue/es/drawer'
import Dropdown from 'ant-design-vue/es/dropdown'
import Empty from 'ant-design-vue/es/empty'
import Form from 'ant-design-vue/es/form'
import Input from 'ant-design-vue/es/input'
import InputNumber from 'ant-design-vue/es/input-number'
import Layout from 'ant-design-vue/es/layout'
import Menu from 'ant-design-vue/es/menu'
import Modal from 'ant-design-vue/es/modal'
import Pagination from 'ant-design-vue/es/pagination'
import Popconfirm from 'ant-design-vue/es/popconfirm'
import Popover from 'ant-design-vue/es/popover'
import Progress from 'ant-design-vue/es/progress'
import Radio from 'ant-design-vue/es/radio'
import Result from 'ant-design-vue/es/result'
import Segmented from 'ant-design-vue/es/segmented'
import Select from 'ant-design-vue/es/select'
import Skeleton from 'ant-design-vue/es/skeleton'
import Spin from 'ant-design-vue/es/spin'
import Switch from 'ant-design-vue/es/switch'
import Table from 'ant-design-vue/es/table'
import Tabs from 'ant-design-vue/es/tabs'
import Tag from 'ant-design-vue/es/tag'
import Timeline from 'ant-design-vue/es/timeline'
import Tooltip from 'ant-design-vue/es/tooltip'

const components = [
  AntdApp,
  Badge,
  Button,
  Card,
  Checkbox,
  ConfigProvider,
  Descriptions,
  Divider,
  Drawer,
  Dropdown,
  Empty,
  Form,
  Input,
  InputNumber,
  Layout,
  Menu,
  Modal,
  Pagination,
  Popconfirm,
  Popover,
  Progress,
  Radio,
  Result,
  Segmented,
  Select,
  Skeleton,
  Spin,
  Switch,
  Table,
  Tabs,
  Tag,
  Timeline,
  Tooltip,
]

export function installAntDesignVue(app: App) {
  for (const component of components) {
    if (component.install) {
      app.use(component)
    }
  }
}
