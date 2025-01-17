import type { Sink } from 'it-stream-types'
import { pushable } from 'it-pushable'

import type { CallData, CallStart } from './rpcproto.pb.js'
import { Packet } from './rpcproto.pb.js'

// CommonRPC is common logic between server and client RPCs.
export class CommonRPC {
  // sink is the data sink for incoming messages.
  public sink: Sink<Packet>
  // source is the packet source for outgoing Packets.
  public source: AsyncIterable<Packet>
  // _source is used to write to the source.
  private readonly _source: {
    push: (val: Packet) => void
    end: (err?: Error) => void
  }
  // rpcDataSource emits incoming client RPC messages to the caller.
  public readonly rpcDataSource: AsyncIterable<Uint8Array>
  // _rpcDataSource is used to write to the rpc message source.
  private readonly _rpcDataSource: {
    push: (val: Uint8Array) => void
    end: (err?: Error) => void
  }

  // service is the rpc service
  protected service?: string
  // method is the rpc method
  protected method?: string

  constructor() {
    this.sink = this._createSink()

    const sourcev = this._createSource()
    this.source = sourcev
    this._source = sourcev

    const rpcDataSource = this._createRpcDataSource()
    this.rpcDataSource = rpcDataSource
    this._rpcDataSource = rpcDataSource
  }

  // writeCallData writes the call data packet.
  public async writeCallData(
    data?: Uint8Array,
    complete?: boolean,
    error?: string
  ) {
    const callData: CallData = {
      data: data || new Uint8Array(0),
      dataIsZero: !!data && data.length === 0,
      complete: complete || false,
      error: error || '',
    }
    await this.writePacket({
      body: {
        $case: 'callData',
        callData,
      },
    })
  }

  // writeCallCancel writes the call cancel packet.
  public async writeCallCancel() {
    await this.writePacket({
      body: {
        $case: 'callCancel',
        callCancel: true,
      },
    })
  }

  // writeCallDataFromSource writes all call data from the iterable.
  public async writeCallDataFromSource(dataSource: AsyncIterable<Uint8Array>) {
    try {
      for await (const data of dataSource) {
        await this.writeCallData(data)
      }
      await this.writeCallData(undefined, true)
    } catch (err) {
      this.close(err as Error)
    }
  }

  // writePacket writes a packet to the stream.
  protected async writePacket(packet: Packet) {
    this._source.push(packet)
  }

  // handleMessage handles an incoming encoded Packet.
  //
  // note: closes the stream if any error is thrown.
  public async handleMessage(message: Uint8Array) {
    return this.handlePacket(Packet.decode(message))
  }

  // handlePacket handles an incoming packet.
  //
  // note: closes the stream if any error is thrown.
  public async handlePacket(packet: Partial<Packet>) {
    try {
      switch (packet?.body?.$case) {
        case 'callStart':
          await this.handleCallStart(packet.body.callStart)
          break
        case 'callData':
          await this.handleCallData(packet.body.callData)
          break
      }
    } catch (err) {
      let asError = err as Error
      if (!asError?.message) {
        asError = new Error('error handling packet')
      }
      this.close(asError)
      throw asError
    }
  }

  // handleCallStart handles a CallStart packet.
  public async handleCallStart(packet: Partial<CallStart>) {
    // no-op
    throw new Error(
      `unexpected call start: ${packet.rpcService}/${packet.rpcMethod}`
    )
  }

  // pushRpcData pushes incoming rpc data to the rpc data source.
  protected pushRpcData(
    data: Uint8Array | undefined,
    dataIsZero: boolean | undefined
  ) {
    if (dataIsZero) {
      if (!data || data.length !== 0) {
        data = new Uint8Array(0)
      }
    } else if (!data || data.length === 0) {
      return
    }
    this._rpcDataSource.push(data)
  }

  // handleCallData handles a CallData packet.
  public async handleCallData(packet: Partial<CallData>) {
    if (!this.service || !this.method) {
      throw new Error('call start must be sent before call data')
    }

    this.pushRpcData(packet.data, packet.dataIsZero)
    if (packet.error) {
      this._rpcDataSource.end(new Error(packet.error))
    } else if (packet.complete) {
      this._rpcDataSource.end()
    }
  }

  // close closes the call, optionally with an error.
  public async close(err?: Error) {
    try {
      await this.writeCallCancel()
    } finally {
      this._rpcDataSource.end(err)
      // note: don't pass error to _source here.
      this._source.end()
    }
  }

  // _createSink returns a value for the sink field.
  private _createSink(): Sink<Packet> {
    return async (source) => {
      try {
        for await (const msg of source) {
          await this.handlePacket(msg)
        }
      } catch (err) {
        const anyErr = err as any
        if (
          anyErr?.code !== 'ERR_STREAM_RESET' &&
          anyErr?.code !== 'ERR_STREAM_ABORT'
        ) {
          this.close(err as Error)
        }
      }
      this._rpcDataSource.end()
    }
  }

  // _createSource returns a value for the source field.
  private _createSource() {
    return pushable<Packet>({
      objectMode: true,
    })
  }

  // _createRpcDataSource returns a value for the rpc data source field.
  private _createRpcDataSource() {
    return pushable<Uint8Array>({
      objectMode: true,
    })
  }
}
