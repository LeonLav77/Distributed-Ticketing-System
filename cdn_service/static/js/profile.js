import CONTRACT_ABI from './MintNFTABI.js';
import { uploadToIPFS } from './IPFSService.js';
import { TICKET_CONTRACT_ADDRESS, connectMetaMask, getWeb3 } from './shared.js';

const AGGREGATOR_URL = '/api';

const PERFORMER_ADDRESSES = [
    "0xe7b317EAe5D7A1C7E3B07Cd70477b62fA2D7AfEE",
    "0xD0558A2d445EC0Dfaef2222eF5693A944Ff124D8",
    "0xE2f12Eb9c45180F150600EbD456f38CA3bDBc610",
    "0x260d25efAD258Ea11eeA26Da09C10b1e02d3c9F0"
];

function profileApp() {
    return {
        loading: true,
        error: null,
        userId: null,
        orders: [],
        mintingOrderId: null,
        mintingStatus: '',

        init() {
            this.fetchProfile();
        },

        async fetchProfile() {
            try {
                this.loading = true;
                this.error = null;

                const token = localStorage.getItem('auth_token');
                const response = await fetch(`${AGGREGATOR_URL}/profile`, {
                    headers: { 'Authorization': `Bearer ${token}` }
                });

                if (response.status === 401) {
                    localStorage.removeItem('auth_token');
                    window.location.href = '/login';
                    return;
                }

                if (!response.ok) throw new Error(`HTTP ${response.status}`);

                const data = await response.json();
                this.userId = data.user_id;
                this.orders = data.orders || [];

                await this.checkMintedStatus();
            } catch (err) {
                this.error = err.message;
            } finally {
                this.loading = false;
            }
        },

        async checkMintedStatus() {
            if (typeof window.ethereum === 'undefined') return;

            try {
                const web3 = getWeb3();
                const contract = new web3.eth.Contract(CONTRACT_ABI, TICKET_CONTRACT_ADDRESS);

                const completedOrders = this.orders.filter(
                    order => order.status === 'completed' && order.tickets?.length > 0
                );

                for (const order of completedOrders) {
                    order.minted = await contract.methods.ticketMinted(order.order_reference_id).call();
                }
            } catch (err) {
                console.error('Error checking minted status:', err);
            }
        },

        getPerformerAddress(performerName) {
            let total = 0;
            for (const char of performerName) {
                total += char.charCodeAt(0);
            }
            return PERFORMER_ADDRESSES[total % PERFORMER_ADDRESSES.length];
        },

        async mintOrder(order) {
            this.mintingOrderId = order.order_reference_id;
            this.mintingStatus = 'Connecting wallet...';

            const userAddress = await connectMetaMask();

            this.mintingStatus = 'Uploading metadata to IPFS...';

            const metadataList = order.tickets.map(ticket => ({
                name: `${order.event.performer_name} - ${ticket.ticket_type.toUpperCase()} Ticket`,
                description: `Seat ${ticket.seat_number} at ${order.event.venue_name}`,
                image: order.event.display_image || '',
                attributes: [
                    { trait_type: "Event", value: order.event.performer_name },
                    { trait_type: "Venue", value: order.event.venue_name },
                    { trait_type: "Seat Number", value: ticket.seat_number },
                    { trait_type: "Ticket Type", value: ticket.ticket_type },
                    { trait_type: "Original Order", value: order.order_reference_id },
                    { trait_type: "Ticket ID", value: ticket.id }
                ]
            }));

            const ipfsUrls = await Promise.all(metadataList.map(uploadToIPFS));

            this.mintingStatus = 'Preparing blockchain transaction...';
            console.log(order);
            const performerName = order.event?.performer_name || 'default';
            const performerAddress = this.getPerformerAddress(performerName);
            console.log(`Using performer address: ${performerAddress} for performer: ${performerName}`);

            this.mintingStatus = 'Minting NFTs on blockchain...';

            const web3 = getWeb3();
            const contract = new web3.eth.Contract(CONTRACT_ABI, TICKET_CONTRACT_ADDRESS);

            const receipt = await contract.methods.mintBatch(
                userAddress,
                ipfsUrls,
                order.order_reference_id,
                performerAddress
            ).send({ from: userAddress });

            const transferEvents = receipt.events?.Transfer;
            let tokenIds = [];
            if (transferEvents) {
                tokenIds = Array.isArray(transferEvents) ? transferEvents.map(e => e.returnValues.tokenId) : [transferEvents.returnValues.tokenId];
            }

            alert(`Successfully minted NFT tickets!\n\nToken IDs: ${tokenIds.length > 0 ? tokenIds.join(', ') : 'Check transaction'}\nPerformer: ${performerAddress}\n\nTransaction: ${receipt.transactionHash}`);
            await this.fetchProfile();
        }
    }
}

function handleLogout() {
    localStorage.removeItem('auth_token');
    document.cookie = 'auth_token=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT';
    window.location.href = '/';
}

window.profileApp = profileApp;
window.handleLogout = handleLogout;