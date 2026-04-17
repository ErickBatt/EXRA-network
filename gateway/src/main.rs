use futures_util::{StreamExt, stream::SplitStream};
use tokio::net::{TcpListener, TcpStream};
use tokio_tungstenite::{accept_async, tungstenite::protocol::Message, WebSocketStream};
use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use log::{info, error};

type SharedSessions = Arc<Mutex<HashMap<String, WebSocketStream<TcpStream>>>>;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    env_logger::init();
    let addr = "0.0.0.0:8082";
    let listener = TcpListener::bind(addr).await?;
    info!("EXRA Rust Gateway listening on: {}", addr);

    let sessions: SharedSessions = Arc::new(Mutex::new(HashMap::new()));

    while let Ok((stream, _)) = listener.accept().await {
        let sessions = sessions.clone();
        tokio::spawn(async move {
            if let Err(e) = handle_connection(stream, sessions).await {
                error!("Error handling connection: {}", e);
            }
        });
    }

    Ok(())
}

async fn handle_connection(stream: TcpStream, sessions: SharedSessions) -> Result<(), Box<dyn std::error::Error>> {
    let ws_stream = accept_async(stream).await?;
    info!("New WebSocket connection");

    // In a real implementation, we would parse the path/query for JWT
    // For this POC, we'll assume the first message is the AUTH {jwt, role}
    let (mut ws_sender, mut ws_receiver) = ws_stream.split();

    if let Some(msg) = ws_receiver.next().await {
        let msg = msg?;
        if msg.is_text() {
            let auth: serde_json::Value = serde_json::from_str(msg.to_text()?)?;
            let jwt = auth["jwt"].as_str().unwrap_or_default().to_string();
            let role = auth["role"].as_str().unwrap_or_default();

            if role == "node" {
                info!("Node registered for session: {}", jwt);
                let mut s = sessions.lock().unwrap();
                // Store the node's stream half (we'll need the whole stream for bridging)
                // Actually, for a blind bridge we need to pair them.
                // Simplified: if buyer is already waiting, bridge them.
                // If not, wait for buyer.
            } else if role == "buyer" {
                info!("Buyer connected for session: {}", jwt);
                // Pair with node and bridge
            }
        }
    }

    Ok(())
}

// TODO: Full bidirectional bridge implementation using tokio::io::copy or manual loop
